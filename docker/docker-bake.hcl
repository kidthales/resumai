variable "APP_ENV" { default = "dev" }
variable "IMAGES_PREFIX" { default = "resumai" }
variable "IMAGES_TAG" { default = "latest" }

function "tag" {
    params = [service_name]
    result = [equal("prod", APP_ENV) ? "${IMAGES_PREFIX}:${service_name}-${IMAGES_TAG}" : "${IMAGES_PREFIX}:${APP_ENV}-${service_name}-${IMAGES_TAG}"]
}

group "default" {
    targets = ["php", "node", "ollama"]
}

target "php" {
    context = ".."
    dockerfile = "docker/php/Dockerfile"
    tags = tag("php")
    target = "app_${APP_ENV}"
}


target "node" {
    context = ".."
    dockerfile = "docker/node/Dockerfile"
    tags = tag("node")
    target = "app_${APP_ENV}"
}

target "ollama" {
    context = ".."
    dockerfile = "docker/ollama/Dockerfile"
    tags = tag("ollama")
    target = "app_${APP_ENV}"
}
