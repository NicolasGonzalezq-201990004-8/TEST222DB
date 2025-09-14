docker-michael:
	docker build -t michael:lab ./michael
	docker run -d --rm --name michael \
	  -e LESTER_ADDR=10.35.168.15:50051 \
	  -e FRANKLIN_ADDR=10.35.168.16:50052 \
	  -e TREVOR_ADDR=10.35.168.17:50053 \
	  michael:lab
