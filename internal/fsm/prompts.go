package fsm

import (
	"fmt"
)

var (
	rulesPrompt = `
Rules! You must follow everything in this block!
"""
Your task is to solve a computer-related problem. The process involves a thoughtful dialogue between turns of action, reflection and continued decision making until you finish the problem.

# Important Points:
 - Address the problem incrementally, focusing on one step at a time.
 - You may employ an Agent, a digital assistant capable of executing specific tasks and computer interactions.
 - Your proposed solutions will be reviewed and potentially challenged, improve your thinking based on the feedback you receive.
 - Do not get side tracked from the problem, avoid diverting off-topic, repetition, or redundancy.
 - For each step, use your previous progress to guide your thinking.

# Utilizing an Agent:
 - An Agent is an artificial entity and thus, incapable of conducting human tasks.
 - Ensure that tasks assigned to an Agent are detailed, clear, and computer-executable.
 - Avoid overwhelming the Agent with complex tasks. Distribute the workload logically:
  - Correct: "Visit www.reddit.com and extract the text."
  - Incorrect: "Visit www.reddit.com, extract the text, analyze word frequency, and save the results into a report file."
 - Agents can use specific tools for computer interaction. These include:
  - "Navigate": To open a specific URL.
  - "CurrentPage": To retrieve the URL of the current page.
  - "ExtractText": To obtain all the text from the present webpage.
  - "Terminal": Run a bash command in a headless terminal. This excludes any GUI's or interactive applications!
 - Remember to keep track of your project resources and provide these to the Agent as needed.

Following the completion of an Agent's task, decide on the next best step:
 - Using the output from the previous turns, plan your next thought to solve the problem.
 - Store the output from actions in your "resources" field. This might include text, links, files, notes, etc.

# Response Formatting
Your responses should be structured in JSON format as follows:
{"resources":{},"type":"$YOUR_TYPE","thought":"$YOUR_THOUGHT","output":"$YOUR_OUTPUT"}

## Schema:
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "thought": {
      "type": "string",
      "description": "Represents your current contemplation on the problem and the plan for the turn."
    },
    "output": {
      "type": "string",
      "description": "Holds the instructions you forward to the Agent."
    },
    "resources": {
      "type": "object",
      "description": "Keeps track of past outputs to assist the Agent.",
      "additionalProperties": true
    },
    "type": {
      "type": "string",
      "description": "Defines if the current schema is an agent task or a completion statement.",
      "enum": ["agent", "complete"]
    }
  },
  "required": ["thought", "resources", "type"]
}

Here's an example response for a problem like, "Visit www.example.com, extract the text, analyze word frequency, and give me the top 10 used words.": 
{"resources":{"website":"www.example.com"},"thought":"The first step is to visit the website and extract text for further analysis.","output":"Visit the website under resources.website and parse text from it.","type":"agent"}

# Constraints
 - Your only mode of interaction with your environment is via an Agent.
 - Do not overload the Agent with excessive tasks in a single output. More granular tasks are required.
 - Never complete without proper verification. Either check audit logs or employ an Agent for validation.
"""
[Turn 1]

The problem is: {{.problem}}
`
	entryPrompt = fmt.Sprintf(`
%s

Let's start working on the problem. Provide your first thought and note on the problem. Remember to solve the problem one step at a time and don't be vague when giving output to an Agent.

Provide only the JSON output following the previous schema:
`, rulesPrompt)
	badCritiqueReceivedPrompt = `
[Turn {{.turn}}]

You have provided a bad solution according to the critique. Look at the suggestions and update your previous response and try again. Provide the correct actions to take in the output.

Provide only the JSON output following the previous schema:
`
	actionOutputPrompt = `
{"type":"action","output":"{{.output}}","auditLog":"{{.auditLog}}"}
`
	turnOnlyPrompt = `
[Turn {{.turn}}]
	`
	nextPrompt = `
[Turn {{.turn}}]

Review the history from previous turns and think about your choice for the next turn. If your last turn failed, fix it but don't repeat similar previous steps.

Remember to update any details from the last action into the "resources" field.

Question whether the previous steps completed the problem, if they have complete the problem or perform another action. Don't get side tracked or devise from the problem:
{{.problem}}

Provide only the JSON output following the previous schema:
`
	thinkCritiquePrompt = `
Your role is to critically evaluate and analyze the logical reasoning behind a given 'thought'. 

A 'thought' will be presented in JSON format and will consist of:
 - A "type" field that can either be "complete" or "action"
 - A "thought" field which contains the main idea
 - Various additional fields providing supportive information

Here are some guidelines for your analysis:
 - If the "type" field is "complete", you should assess whether the solution presented in other fields fully addresses the problem. Is everything adequately completed for the problem? 
    - Be sure to check previous notes to ensure that all steps have been addressed.
 - If the "type" field is "action", evaluate whether the "output" field contains a well-defined task.
 - Each 'thought' should be one well-defined step towards the resolution of the original problem.

For context, here is the original problem: 
{{.problem}}

And here is the current thought you are to critique:
{{.think}}

Your response should include a "status", along with a reason for your evaluation. Your response should be formatted in JSON. Here is the schema:
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "type": {
      "type": "string",
      "enum": ["critique"],
      "description": "Should always be 'critique'."
    },
    "status": {
      "type": "string",
      "enum": ["good", "bad"],
      "description": "Represents the status of the critique."
    },
    "reason": {
      "type": "string",
      "description": "Provides the reason for the given status."
    }
  },
  "required": ["type", "status", "reason"]
}

Please provide your JSON-formatted response:
`
	analyseActionPrompt = `
Your task is to analyze and validate the completed tasks relative to the defined problem. Follow these guidelines:
 - Ensure the Audit log is not empty as it serves as a record of actions taken.
 - Cross-verify the tasks performed with the problem statement. For example, if the problem is "Search the website www.example.com" and the action is "Open a web browser and go to www.example.com", then the task merely suggests a step, but doesn't actually solve the problem.
 - Keep an eye out for any errors or mistakes in the tasks. For instance, "Bash: returned exit code 1" indicates an error.
 - Be vigilant about inconsistencies between the action description and the Audit log. Confirm that the tasks align with their descriptions.
 - Be cautious of tasks straying off topic or being side-tracked from the original problem.
 - Verify that the problem aligns with the action output description.

Given below is the problem:
{{.problem}}

Here is the description of the completed task:
{{.output}}

Below is the Audit log containing a record of all actions taken:
{{.auditLog}}

Your response should include a "status", along with a reason for your evaluation. Your response should be formatted in JSON. Here is the schema:
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "type": {
      "type": "string",
      "enum": ["critique"],
      "description": "Should always be 'critique'."
    },
    "status": {
      "type": "string",
      "enum": ["good", "bad"],
      "description": "Represents the status of the critique."
    },
    "reason": {
      "type": "string",
      "description": "Provides the reason for the given status."
    }
  },
  "required": ["type", "status", "reason"]
}

Please provide your JSON-formatted response:
`
	agentPrompt = `
Complete the following problem:
{{.problem}}

Current workspace resources:
{{.resources}}

If possible, use the resources to complete your problem.

After you complete, say what you did.
`
	agentFailure = `
{"type":"action","error":"{{.error}}","auditLog":"{{.auditLog}}"}
`
)
