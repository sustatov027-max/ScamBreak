build:
	go build -o bin/FraudService ./FraudService/cmd/service
	go build -o bin/NotificationService ./NotificationService/cmd/service
	go build -o bin/PaymentService ./PaymentService/cmd/service
	go build -o bin/UserService ./UserService/cmd/service

start:
	go run ./FraudService/cmd/service
	go run ./NotificationService/cmd/service
	go run ./PaymentService/cmd/service
	go run ./UserService/cmd/service