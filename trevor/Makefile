docker-trevor:
	docker build -t trevor:lab ./trevor
	docker run -d --rm --name trevor \
	  -e AMQP_URL=amqp://guest:guest@10.35.168.15:5672/ \
	  -p 50053:50053 trevor:lab
