FROM golang:1.22 as builder

WORKDIR /app

COPY . .

RUN go build -o gomaster .

CMD ["./gomaster"]
