# This is a sandbox for running the application in
# Bullseye is required for playwright to work
FROM debian:bullseye
RUN echo "deb http://ftp.us.debian.org/debian buster main non-free" >> /etc/apt/sources.list.d/fonts.list
RUN apt update && \
    apt install -y curl

# Download Go
WORKDIR /app
RUN curl https://storage.googleapis.com/golang/go1.20.5.linux-amd64.tar.gz -o go.tar.gz && \
    tar -zxf go.tar.gz && \
    rm -rf go.tar.gz && \
    mv go /go

# Set Go path
ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Download Playwright
RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest

# Install Playwright
RUN playwright install --with-deps

# Expose port to stream to UI
EXPOSE 8080