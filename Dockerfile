FROM golang:1.22.5-alpine3.20

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . .
RUN templ generate
RUN go build -o app .

CMD ["app"]
