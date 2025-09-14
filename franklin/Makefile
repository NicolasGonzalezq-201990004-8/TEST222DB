# ====== VM2: Franklin ======
VM1_IP     ?= 10.35.168.15
AMQP_PORT  ?= 5673
AMQP_USER  ?= heist
AMQP_PASS  ?= heist123

IMAGE      ?= franklin:lab
NAME       ?= franklin
GRPC_PORT  ?= 50052

build:
	docker build -t $(IMAGE) ./franklin

up: stop build
	docker run -d --name $(NAME) \
	  -e AMQP_URL=amqp://$(AMQP_USER):$(AMQP_PASS)@$(VM1_IP):$(AMQP_PORT)/ \
	  -e FRANKLIN_PORT=$(GRPC_PORT) \
	  -p $(GRPC_PORT):$(GRPC_PORT) \
	  $(IMAGE)

logs:
	docker logs -f $(NAME)

inspect-env:
	docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' $(NAME) | grep AMQP_URL || true

stop:
	- docker rm -f $(NAME)

