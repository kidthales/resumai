<?php

/*
 * ResumAI
 * Copyright (C) 2026  Tristan Bonsor
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

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
