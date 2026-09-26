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

namespace App\AI\Agent;

use App\AI\Platform\Result\Processor;
use Symfony\AI\Agent\AgentInterface;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\TextResult;
use Symfony\Component\DependencyInjection\Attribute\Target;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class ResumeDrafter implements ResumeDrafterInterface
{
    public function __construct(
        #[Target('resume_drafter')]
        private AgentInterface $agent,
        private Processor $resultProcessor,
    ) {
    }

    public function draft(
        ?string $jobDescription = null,
        ?string $candidateArchetype = null,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $textDeltaProcessor = null,
    ): string {
        $prompt = $this->buildPrompt($jobDescription, $candidateArchetype);
        $execution = $this->agent->call($prompt);

        $resumeText = '';
        $thinkingText = '';

        $this->resultProcessor->process(
            $execution->getResult(),
            textResultProcessor: static function (TextResult $result) use (&$resumeText): void {
                $resumeText .= (string) $result->getContent();
            },
            textDeltaProcessor: static function (TextDelta $delta) use (&$resumeText, $textDeltaProcessor): void {
                $resumeText .= $delta->getText();
                if (null !== $textDeltaProcessor) {
                    $textDeltaProcessor($delta);
                }
            },
            thinkingDeltaProcessor: static function (ThinkingDelta $delta) use (&$thinkingText, $thinkingDeltaProcessor): void {
                $thinkingText .= $delta->getThinking();
                if (null !== $thinkingDeltaProcessor) {
                    $thinkingDeltaProcessor($delta);
                }
            },
        );

        return $this->sanitizeOutput($resumeText);
    }

    private function buildPrompt(?string $jobDescription, ?string $candidateArchetype): string
    {
        $parts = [
            'Synthesize a complete, high-impact resume in Markdown format based on the candidate profile.',
        ];

        if (null !== $jobDescription && '' !== trim($jobDescription)) {
            $parts[] = "Target the resume specifically to the following Job Description:\n<job_description>\n".trim($jobDescription)."\n</job_description>";
        }

        if (null !== $candidateArchetype && '' !== trim($candidateArchetype)) {
            $parts[] = "Position the candidate according to the following Candidate Archetype:\n<candidate_archetype>\n".trim($candidateArchetype)."\n</candidate_archetype>";
        }

        $parts[] = 'Output only the final Markdown resume. Do not include introductory text, explanations, or enclosing code block fences.';

        return implode("\n\n", $parts);
    }

    private function sanitizeOutput(string $output): string
    {
        $trimmed = trim($output);

        if (preg_match('/^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$/s', $trimmed, $matches)) {
            return trim($matches[1]);
        }

        return $trimmed;
    }
}
