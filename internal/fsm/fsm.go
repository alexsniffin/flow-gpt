package fsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	customAgent "flow-gpt/internal/agent"
	customIntegration "flow-gpt/internal/integration"
	customTool "flow-gpt/internal/tool"
	"github.com/cenkalti/backoff"
	"github.com/gorilla/websocket"
	"github.com/hupe1980/golc"
	"github.com/hupe1980/golc/agent"
	"github.com/hupe1980/golc/model"
	"github.com/hupe1980/golc/model/chatmodel"
	"github.com/hupe1980/golc/prompt"
	"github.com/hupe1980/golc/schema"
	"github.com/hupe1980/golc/tool"
	"github.com/hupe1980/golc/toolkit"
	"github.com/playwright-community/playwright-go"
	zLog "github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"
)

const (
	AgentTimeout = 30 * time.Second
	ChatTimeout  = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type State interface{}

type FSM struct {
	openaiChat    *chatmodel.OpenAI
	actionAgent   *agent.Executor
	Browser       playwright.Browser
	thinkMessages schema.ChatMessages
	problem       string
	turn          int
	tokensUsed    int
	state         State
	stream        chan string
}

func New(problem string, turn int) (*FSM, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	browser, err := pw.Chromium.Launch()
	if err != nil {
		return nil, err
	}

	browserKit, err := toolkit.NewBrowser(browser)
	if err != nil {
		return nil, err
	}

	var tools []schema.Tool
	tools = append(tools, browserKit.Tools()...)
	tools = append(tools, tool.NewSleep())
	tools = append(tools, customTool.NewTerminal(customIntegration.NewBashProcess()))

	openaiChat, err := chatmodel.NewOpenAI(os.Getenv("OPENAI_API_KEY"), func(o *chatmodel.OpenAIOptions) {
		o.ModelName = "gpt-3.5-turbo-16k"
		o.Temperature = 0.05
	})
	if err != nil {
		return nil, err
	}

	actionAgent, err := agent.NewOpenAIFunctions(openaiChat, tools)
	if err != nil {
		return nil, err
	}

	stream := make(chan string, 1)
	stream <- "Problem: " + problem
	return &FSM{
		openaiChat:    openaiChat,
		actionAgent:   actionAgent,
		Browser:       browser,
		thinkMessages: schema.ChatMessages{},
		problem:       problem,
		turn:          turn,
		state:         Init{},
		stream:        stream,
	}, nil
}

func (fsm *FSM) Process(ctx context.Context) {
	for {
		zLog.Info().Msg("turn: " + fmt.Sprint(fsm.turn))
		select {
		case <-ctx.Done():
			zLog.Info().Msg("shutting down state loop")
			return
		default:
			var err error
			zLog.Info().Msgf("state: %v", fsm.state)
			switch msg := fsm.state.(type) {
			case Action:
				err = fsm.HandleActionState(msg)
			case Next:
				err = fsm.HandleNextThought(msg)
				fsm.turn++
			case Complete:
				err = fsm.HandleCompleteState(msg)
				return
			case Init:
				err = fsm.HandleInitState(msg)
				fsm.turn++
			case JudgeAction:
				err = fsm.HandleJudgeActionState(msg)
			case JudgeThought:
				err = fsm.HandleJudgeThoughtState(msg)
			case ThoughtDecider:
				err = fsm.HandleThoughtDeciderState(msg)
				fsm.turn++
			default:
				zLog.Fatal().Msg("unknown message")
			}
			if err != nil {
				zLog.Error().Msgf("failed to handle state: %v", err)
				return
			}
		}
	}
}

func (fsm *FSM) SetState(state State) {
	fsm.state = state
}

func (fsm *FSM) HandleNextThought(state Next) error {
	zLog.Debug().Msgf("state content: %v", state)
	f := prompt.NewSystemMessageTemplate(nextPrompt)
	p, err := f.Format(map[string]any{
		"turn":    fsm.turn,
		"problem": fsm.problem,
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}
	tPrompt, err := nextTurnPrompt(fsm.turn)
	if err != nil {
		return fmt.Errorf("failed to render turn prompt: %w", err)
	}
	res, err := fsm.ChatGenerate(context.Background(), append(fsm.thinkMessages, schema.ChatMessages{p}...))
	if err != nil {
		return fmt.Errorf("failed to call chain: %w", err)
	}
	fsm.stream <- res.Content()
	fsm.appendThinkChat(tPrompt, res)
	fsm.SetState(JudgeThought{Message: res.Content()})
	return nil
}

func (fsm *FSM) HandleActionState(state Action) error {
	zLog.Debug().Msgf("state content: %v", state)
	f := prompt.NewFormatter(agentPrompt)
	p, err := f.Render(map[string]any{
		"problem":   state.Output,
		"resources": state.Resources,
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), AgentTimeout)
	defer cancel()
	res, auditLog, err := fsm.AgentGenerate(ctx, p)
	if err != nil {
		var bashErr customIntegration.BashProcessError
		if errors.As(err, &bashErr) {
			resF := prompt.NewSystemMessageTemplate(agentFailure)
			actionRes, err := resF.Format(map[string]any{
				"error":    bashErr.Error(),
				"auditLog": auditLog,
			})
			if err != nil {
				return fmt.Errorf("failed to render prompt: %w", err)
			}
			fsm.stream <- actionRes.Content()
			fsm.appendThinkChat(actionRes)
			fsm.SetState(JudgeAction{Problem: state.Output, Message: agentFailure + bashErr.Error(), AuditLog: auditLog})
			return nil
		} else {
			return fmt.Errorf("failed to call agent: %w", err)
		}
	}
	resF := prompt.NewSystemMessageTemplate(actionOutputPrompt)
	actionRes, err := resF.Format(map[string]any{
		"output":   escape(res),
		"auditLog": escape(auditLog),
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}
	fsm.stream <- actionRes.Content()
	fsm.appendThinkChat(actionRes)
	fsm.SetState(JudgeAction{Problem: state.Output, Message: res, AuditLog: auditLog})
	return nil
}

func (fsm *FSM) HandleCompleteState(state Complete) error {
	zLog.Debug().Msgf("state content: %v", state)
	fsm.stream <- fmt.Sprintf("Completed! Tokens used: %d", fsm.tokensUsed)
	return nil
}

func (fsm *FSM) HandleInitState(state Init) error {
	zLog.Debug().Msgf("state content: %v", state)
	f := prompt.NewSystemMessageTemplate(entryPrompt)
	p, err := f.Format(map[string]any{
		"problem": fsm.problem,
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}
	rTemplate := prompt.NewSystemMessageTemplate(rulesPrompt)
	rFormat, err := rTemplate.Format(map[string]any{})
	if err != nil {
		return fmt.Errorf("failed to render rules prompt: %w", err)
	}
	res, err := fsm.ChatGenerate(context.Background(), schema.ChatMessages{p})
	if err != nil {
		return fmt.Errorf("failed to call chain: %w", err)
	}
	fsm.stream <- res.Content()
	fsm.appendThinkChat(rFormat, res)
	fsm.SetState(JudgeThought{Message: res.Content()})
	return nil
}

func (fsm *FSM) HandleJudgeActionState(state JudgeAction) error {
	zLog.Debug().Msgf("state content: %v", state)
	f := prompt.NewSystemMessageTemplate(analyseActionPrompt)
	p, err := f.Format(map[string]any{
		"problem":  state.Problem,
		"output":   state.Message,
		"auditLog": state.AuditLog,
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ChatTimeout)
	defer cancel()
	res, err := fsm.ChatGenerate(ctx, schema.ChatMessages{p}) // todo wait for gpt-35-turbo-instruct, till then pass 1 message
	if err != nil {
		return fmt.Errorf("failed to call chain: %w", err)
	}
	fsm.stream <- res.Content()
	fsm.appendThinkChat(res)
	fsm.SetState(Next{})
	return nil
}

func (fsm *FSM) HandleJudgeThoughtState(state JudgeThought) error {
	zLog.Debug().Msgf("state content: %v", state)
	f := prompt.NewSystemMessageTemplate(thinkCritiquePrompt)
	p, err := f.Format(map[string]any{
		"problem": fsm.problem,
		"think":   state.Message,
	})
	if err != nil {
		return fmt.Errorf("failed to render prompt: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ChatTimeout)
	defer cancel()
	res, err := fsm.ChatGenerate(ctx, schema.ChatMessages{p}) // todo wait for gpt-35-turbo-instruct, till then pass 1 message
	if err != nil {
		return fmt.Errorf("failed to call chain: %w", err)
	}
	fsm.stream <- res.Content()
	fsm.appendThinkChat(res)
	fsm.SetState(ThoughtDecider{Thought: state.Message, JudgeMessage: res.Content()})
	return nil
}

func (fsm *FSM) HandleThoughtDeciderState(state ThoughtDecider) error {
	zLog.Debug().Msgf("state content: %v", state)
	if gjson.Get(state.JudgeMessage, "status").String() == "good" {
		if gjson.Get(state.Thought, "type").String() == "complete" {
			fsm.SetState(Complete{})
			return nil
		} else if gjson.Get(state.Thought, "type").String() == "agent" {
			aMsg, err := unmarshalAction(state.Thought)
			if err != nil {
				return fmt.Errorf("failed to unmarshal action message: %w", err)
			}
			fsm.SetState(aMsg)
		} else {
			return errors.New("unknown thought type")
		}
	} else if gjson.Get(state.JudgeMessage, "status").String() == "bad" {
		f := prompt.NewSystemMessageTemplate(badCritiqueReceivedPrompt)
		p, err := f.Format(map[string]any{
			"turn": fsm.turn,
		})
		if err != nil {
			return fmt.Errorf("failed to render prompt: %w", err)
		}
		tPrompt, err := nextTurnPrompt(fsm.turn)
		if err != nil {
			return fmt.Errorf("failed to render turn prompt: %w", err)
		}
		res, err := fsm.ChatGenerate(context.Background(), append(fsm.thinkMessages, schema.ChatMessages{p}...))
		if err != nil {
			return fmt.Errorf("failed to call chain: %w", err)
		}
		fsm.stream <- res.Content()
		fsm.appendThinkChat(tPrompt, res)
		fsm.SetState(JudgeThought{Message: res.Content()})
	} else {
		return fmt.Errorf("not implemented message: %v", state)
	}
	return nil
}

func (fsm *FSM) ChatGenerate(ctx context.Context, messages []schema.ChatMessage) (schema.AIChatMessage, error) {
	var result schema.AIChatMessage
	var err error
	operation := func() error {
		r, err := model.ChatModelGenerate(ctx, fsm.openaiChat, messages)
		if err != nil {
			return fmt.Errorf("error calling chain: %w", err)
		}
		zLog.Info().Msgf("token usage: %v", r.LLMOutput)
		fsm.tokensUsed += r.LLMOutput["TokenUsage"].(map[string]int)["TotalTokens"]
		msg, ok := r.Generations[0].Message.(*schema.AIChatMessage)
		if !ok {
			return backoff.Permanent(errors.New("unexpected result type"))
		}
		result = *msg
		return nil
	}

	notify := func(err error, t time.Duration) {
		zLog.Error().Err(err).Msg("Operation failed. Retrying...")
	}

	err = backoff.RetryNotify(operation, backoff.NewConstantBackOff(time.Second), notify)
	if err != nil {
		return schema.AIChatMessage{}, err
	}
	return result, nil
}

func (fsm *FSM) AgentGenerate(ctx context.Context, input string) (string, string, error) {
	var result string
	var auditLog string
	var err error
	operation := func() error {
		aLog := customAgent.NewCallbackAuditLog()
		result, err = golc.SimpleCall(ctx, fsm.actionAgent, input, func(o *golc.SimpleCallOptions) {
			o.Callbacks = []schema.Callback{aLog}
		})
		auditLog = aLog.AuditLog()
		if err != nil {
			var bashErr customIntegration.BashProcessError
			if errors.As(err, &bashErr) {
				return backoff.Permanent(bashErr)
			} else {
				return fmt.Errorf("error calling agent: %w", err)
			}
		}
		return nil
	}

	notify := func(err error, t time.Duration) {
		zLog.Error().Err(err).Msg("Operation failed. Retrying...")
	}

	err = backoff.RetryNotify(operation, backoff.NewConstantBackOff(time.Second), notify)
	if err != nil {
		return "", auditLog, err
	}
	if result == "" {
		return "no output was produced", auditLog, nil
	}
	return result, auditLog, nil
}

func unmarshalAction(action string) (Action, error) {
	r := Action{}
	err := json.Unmarshal([]byte(action), &r)
	if err != nil {
		return Action{}, err
	}
	return r, nil
}

func escape(input string) string {
	escaped, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}

	return string(escaped[1 : len(escaped)-1])
}

func nextTurnPrompt(turn int) (schema.ChatMessage, error) {
	f := prompt.NewSystemMessageTemplate(turnOnlyPrompt)
	p, err := f.Format(map[string]any{
		"turn": turn,
	})
	if err != nil {
		return schema.SystemChatMessage{}, fmt.Errorf("failed to render prompt: %w", err)
	}
	return p, nil
}

func (fsm *FSM) appendThinkChat(chat ...schema.ChatMessage) {
	fsm.thinkMessages = append(fsm.thinkMessages, chat...)
}

func (fsm *FSM) Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade: ", err)
		return
	}
	defer conn.Close()

	for {
		out := <-fsm.stream
		if err = conn.WriteMessage(websocket.TextMessage, []byte(out)); err != nil {
			log.Println("Write: ", err)
			break
		}
	}
}
