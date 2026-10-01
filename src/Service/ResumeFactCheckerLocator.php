<?php

declare(strict_types=1);

namespace App\Service;

final readonly class ResumeFactCheckerLocator extends AgentByPlatformLocator
{
    protected const string AGENT_NAME = 'resume_fact_checker';
}
