IMAGE ?= michael:lab

LESTER_ADDR   ?= 10.35.168.15:50051
FRANKLIN_ADDR ?= 10.35.168.16:50052
TREVOR_ADDR   ?= 10.35.168.17:50053

build:
	docker build -f michael/Dockerfile -t $(IMAGE) .

up: down build
	docker run -d --name michael \
	  -e LESTER_ADDR=$(LESTER_ADDR) \
	  -e FRANKLIN_ADDR=$(FRANKLIN_ADDR) \
	  -e TREVOR_ADDR=$(TREVOR_ADDR) \
	  $(IMAGE)

logs:
	docker logs -f michael

down:
	- docker rm -f michael 2>/dev/null || true
