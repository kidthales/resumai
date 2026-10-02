<?php

declare(strict_types=1);

namespace App\Service;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class ResumeDrafterLocator extends AgentByPlatformLocator
{
    protected const string AGENT_NAME = 'resume_drafter';
}
