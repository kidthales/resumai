<?php

declare(strict_types=1);

namespace App\Service;

final readonly class ResumeDrafterLocator extends AgentByPlatformLocator
{
    protected const string AGENT_NAME = 'resume_drafter';
}
