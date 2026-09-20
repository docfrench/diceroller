FROM golang:latest
WORKDIR /app
COPY . .
RUN go build -o dice_roller
CMD ["./dice_roller"]
