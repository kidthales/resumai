# Executables (local)
CD_DOCKER       = cd docker
DOCKER_COMP     = docker compose
CD_DOCKER_COMP := $(CD_DOCKER) && $(DOCKER_COMP)

# Docker containers
PHP_CONT    := $(CD_DOCKER_COMP) exec php
NODE_CONT   := $(CD_DOCKER_COMP) exec node
OLLAMA_CONT := $(CD_DOCKER_COMP) exec ollama

# Executables
PHP      := $(PHP_CONT) php
COMPOSER := $(PHP_CONT) composer
SYMFONY  := $(PHP) bin/console
NPM      := $(NODE_CONT) npm
NPX      := $(NODE_CONT) npx
OLLAMA   := $(OLLAMA_CONT) ollama

# Misc
.DEFAULT_GOAL = help
.PHONY        : help \
                build up start down logs sh bash test \
                composer vendor \
                sf cc \
                npm node_modules \
                npx \
                lint lint-fix prettier php-cs-fixer \
                pull-models pull-skills ollama skills \
                own

## —— 🎵 🐳 The Symfony Docker Makefile 🐳 🎵 ——————————————————————————————————
help: ## Outputs this help screen
	@grep -E '(^[a-zA-Z0-9\./_-]+:.*?##.*$$)|(^##)' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}{printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

## —— Docker 🐳 ————————————————————————————————————————————————————————————————
build: ## Builds the Docker images
	@$(CD_DOCKER) && touch -a .env && docker buildx bake --allow=fs.read=.. -f .env -f docker-bake.hcl --pull --no-cache

up: ## Start the docker hub in detached mode (no logs)
	@$(CD_DOCKER_COMP) up --detach

start: build up ## Build and start the containers

down: ## Stop the docker hub
	@$(CD_DOCKER_COMP) down --remove-orphans

logs: ## Show live logs
	@$(CD_DOCKER_COMP) logs --tail=0 --follow

sh: ## Connect to the FrankenPHP container
	@$(PHP_CONT) sh

bash: ## Connect to the FrankenPHP container via bash so up and down arrows go to previous commands
	@$(PHP_CONT) bash

test: ## Start tests with phpunit, pass the parameter "c=" to add options to phpunit, example: make test c="--group e2e --stop-on-failure"
	@$(eval c ?=)
	@$(CD_DOCKER_COMP) exec -e APP_ENV=test php bin/phpunit $(c)

## —— Composer 🧙 ——————————————————————————————————————————————————————————————
composer: ## Run composer, pass the parameter "c=" to run a given command, example: make composer c='req symfony/orm-pack'
	@$(eval c ?=)
	@$(COMPOSER) $(c)

vendor: ## Install vendors according to the current composer.lock file
vendor: c=install --prefer-dist --no-dev --no-progress --no-scripts --no-interaction
vendor: composer

## —— Symfony 🎵 ———————————————————————————————————————————————————————————————
sf: ## List all Symfony commands or pass the parameter "c=" to run a given command, example: make sf c=about
	@$(eval c ?=)
	@$(SYMFONY) $(c)

cc: c=c:c ## Clear the cache
cc: sf

## —— NPM 📦️ ——————————————————————————————————————————————————————————————————
npm: ## Run npm, pass the parameter "c=" to run a given command, example: make npm c='i -D prettier'
	@$(eval c ?=)
	@$(NPM) $(c)

node_modules: ## Install node_modules according to the current package-lock.json file
node_modules: c=ci
node_modules: npm

## —— NPX ❌️ ——————————————————————————————————————————————————————————————————
npx: ## Run npx, pass the parameter "c=" to run a given command, example: make npx c='prettier --check .'
	@$(eval c ?=)
	@$(NPX) $(c)

## —— Lint 🧹 —————————————————————————————————————————————————————————————————
lint: ## Check files for lint errors
	-@$(NPX) prettier --check .
	-@$(PHP) vendor/bin/php-cs-fixer check

lint-fix: ## Fix files with lint errors
	-@$(NPX) prettier --write .
	-@$(PHP) vendor/bin/php-cs-fixer fix

prettier:  ## Run prettier, pass the parameter "c=" to run a given command, example: make prettier c='--check .'
	@$(eval c ?=)
	@$(NPX) prettier $(c)

php-cs-fixer: ## Run php-cs-fixer, pass the parameter "c=" to run a given command, example: make php-cs-fixer c='check'
	@$(eval c ?=)
	@$(PHP) vendor/bin/php-cs-fixer $(c)

## —— AI 🤖 ————————————————————————————————————————————————————————————————————
REQUIRED_MODELS = qwen3:14b

pull-models: ## Pull required models for use with ollama
	@$(OLLAMA) list > /dev/null 2>&1 || (echo "❌ Error: Ollama service is not running. Please start Ollama first." && exit 1)
	@for model in $(REQUIRED_MODELS); do \
		echo "📥 Pulling model: $$model..."; \
		$(OLLAMA) pull $$model; \
	done
	@echo "✅ All models successfully installed!"

pull-skills: ## Pull required skills for use with agents
pull-skills: c=experimental_install
pull-skills: skills

ollama: ## Run ollama cli, pass the parameter "c=" to run a given command; example: make ollama c='pull qwen3:14b'
	@$(eval c ?=)
	@$(OLLAMA) $(c)

skills: ## Run skills cli. Pass the parameter "c=" to run a given command; example: make skills c='add phaserjs/phaser'
	@$(eval c ?=)
	@$(NPX) skills $(c)

## —— Troubleshooting 🔎 ———————————————————————————————————————————————————————
own: ## On Linux, set yourself as owner of files created by the Docker container
	@$(CD_DOCKER_COMP) run --quiet --rm php chown -R $$(id -u):$$(id -g) .
