COMPOSE ?= docker-compose
YML     ?= compose.lester.yml

.PHONY: up build logs down ps

build:
	$(COMPOSE) -f $(YML) build

up: build
	$(COMPOSE) -f $(YML) up -d

logs:
	$(COMPOSE) -f $(YML) logs -f

down:
	$(COMPOSE) -f $(YML) down

ps:
	$(COMPOSE) -f $(YML) ps

