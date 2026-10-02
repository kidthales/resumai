<?php

declare(strict_types=1);

namespace App\Service;

use Psr\Container\ContainerInterface;
use Symfony\AI\Agent\AgentInterface;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\DependencyInjection\Attribute\Target;
use Symfony\Contracts\Service\Attribute\SubscribedService;
use Symfony\Contracts\Service\ServiceSubscriberInterface;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
abstract readonly class AgentByPlatformLocator implements ServiceSubscriberInterface
{
    protected const string AGENT_NAME = '';

    public static function getSubscribedServices(): array
    {
        return [
            'ollama' => new SubscribedService(
                type: AgentInterface::class,
                attributes: new Target(\sprintf('%s.ollama', static::AGENT_NAME))
            ),
            'ollama.model' => new SubscribedService(
                type: 'string',
                attributes: new Autowire(param: \sprintf('%s.ollama.model', static::AGENT_NAME))
            ),
            'ollama.model_params' => new SubscribedService(
                type: 'array',
                attributes: new Autowire(param: \sprintf('%s.ollama.model_params', static::AGENT_NAME))
            ),
            'gemini' => new SubscribedService(
                type: AgentInterface::class,
                attributes: new Target(\sprintf('%s.gemini', static::AGENT_NAME))
            ),
            'gemini.model' => new SubscribedService(
                type: 'string',
                attributes: new Autowire(param: \sprintf('%s.gemini.model', static::AGENT_NAME))
            ),
            'gemini.model_params' => new SubscribedService(
                type: 'array',
                attributes: new Autowire(param: \sprintf('%s.gemini.model_params', static::AGENT_NAME))
            ),
        ];
    }

    public function __construct(private ContainerInterface $container)
    {
    }

    public function getAgentByPlatform(string $platform): AgentInterface
    {
        return $this->container->get($platform);
    }

    public function getModelByPlatform(string $platform): string
    {
        return $this->container->get(\sprintf('%s.model', $platform));
    }

    public function getModelParamsByPlatform(string $platform): array
    {
        return $this->container->get(\sprintf('%s.model_params', $platform));
    }
}
