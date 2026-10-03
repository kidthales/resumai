# Executables (local)
AWK             = awk
CD_DOCKER       = cd docker
DOCKER_COMP     = docker compose
CD_DOCKER_COMP := $(CD_DOCKER) && $(DOCKER_COMP)
ECHO            = echo
GREP            = grep
SED             = sed
SORT            = sort

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
.PHONY        : help start fresh-start stop \
                resume-arch resume-draft resume-fc resume-jac resume-edit \
                bake up down logs \
                test cov \
                composer vendor \
                sf cc \
                npm node_modules npx \
                lint fix prettier prettier-check prettier-fix php-cs-fixer php-cs-fixer-check php-cs-fixer-fix \
                ollama ollama-pull skills \
                own

## —— 📄 🤖 The ResumAI Makefile 🤖 📄 —————————————————————————————————————————
help: ## Outputs this help screen
	@$(GREP) -E '(^[a-zA-Z0-9\./_-]+:.*?##.*$$)|(^##)' $(MAKEFILE_LIST) | $(AWK) 'BEGIN {FS = ":.*?## "}{printf "\033[32m%-30s\033[0m %s\n", $$1, $$2}' | $(SED) -e 's/\[32m##/[33m/'

start: bake up logs ## Start the app

fresh-start: ## Start the app with fresh images
	@$(MAKE) bake c='--pull --no-cache' up logs

stop: down ## Stop the app

resume-arch: ## Select resume archetype for a job description, pass the parameter "c=" to specify options and the JD path, example: make resume-arch c='var/jd.txt'
	@$(eval c ?=)
	@$(SYMFONY) app:resume:archetype:select $(c)

resume-draft: ## Draft resume optionally tailored to job description and archetype, pass the parameter "c=" to specify options and the output path, example: make resume-draft c='var/resume_draft.md'
	@$(eval c ?=)
	@$(SYMFONY) app:resume:draft $(c)

resume-fc: ## Fact-check resume, pass the parameter "c=" to specify options and the input/output paths, example: make resume-fc c='var/resume_draft.md var/resume_fc.md'
	@$(eval c ?=)
	@$(SYMFONY) app:resume:fact-check $(c)

resume-jac: ## Check resume for job alignment, pass the parameter "c=" to specify options and the input/output paths, example: make resume-jac c='var/resume_draft.md var/jd.txt var/resume_jac.md'
	@$(eval c ?=)
	@$(SYMFONY) app:resume:job-alignment-check $(c)

resume-edit: ## Edit resume with fact-check and optional job-alignment-check, pass the parameter "c=" to specify options and the input/output paths, example: make resume-edit c='var/resume_draft.md var/resume_fc.md var/resume_edit.md'
	@$(eval c ?=)
	@$(SYMFONY) app:resume:edit $(c)

## —— Docker 🐳 ————————————————————————————————————————————————————————————————
bake: ## Bakes the Docker images, pass the parameter "c=" to specify options and targets, example: make bake c='--pull --no-cache php node'
	@$(eval c ?=)
	@$(CD_DOCKER) && touch -a .env && docker buildx bake --allow=fs.read=.. -f .env -f docker-bake.hcl $(c)

up: ## Start the docker hub in detached mode (no logs)
	@$(CD_DOCKER_COMP) up --detach

down: ## Stop the docker hub
	@$(CD_DOCKER_COMP) down --remove-orphans

logs: ## Show live logs
	@$(CD_DOCKER_COMP) logs --tail=0 --follow

## —— Tests 🧪 —————————————————————————————————————————————————————————————————
test: ## Start tests with phpunit, pass the parameter "c=" to add options to phpunit, example: make test c="--group e2e --stop-on-failure"
	@$(eval c ?=)
	@$(CD_DOCKER_COMP) exec -e APP_ENV=test -e XDEBUG_MODE=coverage php bin/phpunit $(c)

cov: ## ## Start tests with phpunit and generate coverage report for the project
cov: c=--testdox --display-all-issues --coverage-text --show-uncovered-for-coverage-text --coverage-html coverage/html --coverage-jsonl coverage/jsonl --coverage-clover coverage/clover.xml
cov: test

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

## —— Node.js 📦️ ——————————————————————————————————————————————————————————————
npm: ## Run npm, pass the parameter "c=" to run a given command, example: make npm c='i -D prettier'
	@$(eval c ?=)
	@$(NPM) $(c)

node_modules: ## Install node_modules according to the current package-lock.json file
node_modules: c=ci
node_modules: npm

npx: ## Run npx, pass the parameter "c=" to run a given command, example: make npx c='prettier --check .'
	@$(eval c ?=)
	@$(NPX) $(c)

## —— Lint 🧹 ——————————————————————————————————————————————————————————————————
lint: ## Check files for lint errors
	@$(MAKE) -j 2 --output-sync prettier-check php-cs-fixer-check

fix: ## Fix files with lint errors
	@$(MAKE) -j 2 --output-sync prettier-fix php-cs-fixer-fix

prettier: ## Run prettier, pass the parameter "c=" to run a given command, example: make prettier c='--check .'
	@$(eval c ?=)
	@$(NPX) prettier $(c)

prettier-check: ## Check files with prettier
prettier-check: c=--check .
prettier-check: prettier

prettier-fix: ## Fix files with prettier
prettier-fix: c=--write .
prettier-fix: prettier

php-cs-fixer: ## Run php-cs-fixer, pass the parameter "c=" to run a given command, example: make php-cs-fixer c='check'
	@$(eval c ?=)
	@$(PHP) vendor/bin/php-cs-fixer $(c)

php-cs-fixer-check: ## Check files with php-cs-fixer
php-cs-fixer-check: c=check
php-cs-fixer-check: php-cs-fixer

php-cs-fixer-fix: ## Fix files with php-cs-fixer
php-cs-fixer-fix: c=fix
php-cs-fixer-fix: php-cs-fixer

## —— AI 🤖 ————————————————————————————————————————————————————————————————————
ollama: ## Run ollama cli, pass the parameter "c=" to run a given command; example: make ollama c='pull qwen3:14b'
	@$(eval c ?=)
	@$(OLLAMA) $(c)

ollama-pull: ## Pull the agent Ollama models currently assigned in the .env file
	@for model in $$($(GREP) -oP "^OLLAMA_AGENT_.*_MODEL=[\"']?\K.*?(?=[\"']?$$)" .env | $(SORT) -u); do \
		$(ECHO) Pulling $$model...; \
		$(OLLAMA) pull $$model; \
	done

skills: ## Run skills cli. Pass the parameter "c=" to run a given command; example: make skills c='add phaserjs/phaser'
	@$(eval c ?=)
	@$(NPX) skills $(c)

## —— Troubleshooting 🔎 ———————————————————————————————————————————————————————
own: ## On Linux, set yourself as owner of files created by the Docker containers
	@$(CD_DOCKER_COMP) run --quiet --rm php chown -R $$(id -u):$$(id -g) .
