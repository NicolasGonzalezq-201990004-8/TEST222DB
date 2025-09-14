IMAGE ?= michael:lab
NAME  ?= michael

LESTER_ADDR   ?= 10.35.168.15:50051
FRANKLIN_ADDR ?= 10.35.168.17:50052
TREVOR_ADDR   ?= 10.35.168.16:50053

build:
	docker build -t $(IMAGE) -f ./Dockerfile .

up: stop build
	docker run -d --name $(NAME) \
	  -e LESTER_ADDR=$(LESTER_ADDR) \
	  -e FRANKLIN_ADDR=$(FRANKLIN_ADDR) \
	  -e TREVOR_ADDR=$(TREVOR_ADDR) \
	  $(IMAGE)

logs:
	docker logs -f $(NAME)

stop:
	- docker rm -f $(NAME)

