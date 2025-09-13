docker-franklin:
	docker build -t franklin:lab ./franklin
	docker run -d --rm --name franklin \
	  -e AMQP_URL=amqp://guest:guest@10.35.168.15:5672/ \
	  -p 50052:50052 franklin:lab
