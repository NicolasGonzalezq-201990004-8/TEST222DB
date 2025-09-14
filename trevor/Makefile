COMPOSE ?= docker-compose
FILE    ?= docker-compose.trevor.yml
AMQP    ?= amqp://heist:heist123@10.35.168.15:5673/

build:
	$(COMPOSE) -f $(FILE) build

up:
	AMQP_URL='$(AMQP)' $(COMPOSE) -f $(FILE) up -d --build

logs:
	$(COMPOSE) -f $(FILE) logs -f

ps:
	$(COMPOSE) -f $(FILE) ps

down:
	$(COMPOSE) -f $(FILE) down -v
