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

namespace App\Command\Resume;

use App\Command\CallAgentAndStreamExecutionProgressTrait;
use App\Filesystem\Filesystem;
use App\Service\ResumeEditorLocator;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Attribute\Option;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Style\SymfonyStyle;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsCommand(
    name: 'app:resume:edit',
    description: 'Edit a resume with a fact-check report and an optional job-alignment report',
)]
final class EditCommand
{
    use CallAgentAndStreamExecutionProgressTrait;

    public function __construct(
        private readonly ResumeEditorLocator $resumeEditorLocator,
        private readonly Filesystem $filesystem,
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Resume input filepath')] string $resumeInputPath,
        #[Argument('Resume fact-check input filepath')] string $resumeFactCheckInputPath,
        #[Argument('Resume output filepath')] string $resumeOutputPath,
        #[Option('Resume job-alignment-check input filepath', 'job', 'j')] ?string $resumeJobAlignmentCheckInputPath = null,
        #[Option('Agent platform', 'platform', 'p')] string $agentPlatform = 'ollama',
    ): int {
        $io->title('Resume Edit');

        $io->section('Parameters');

        $resume = $this->fetchResume($resumeInputPath);
        $resumeFactCheck = $this->fetchResumeFactCheck($resumeFactCheckInputPath);
        $resolvedResumeOutputPath = $this->filesystem->resolveProjectPath($resumeOutputPath);
        $resumeJobAlignmentCheck = $this->fetchResumeJobAlignmentCheck($resumeJobAlignmentCheckInputPath);
        $normalizedAgentPlatform = u($agentPlatform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume input filepath' => $resumeInputPath],
            ['Resume fact-check input filepath' => $resumeFactCheckInputPath],
            ['Resume output filepath' => $resumeOutputPath],
            ['Resume job-alignment-check input filepath' => null === $resumeJobAlignmentCheckInputPath ? '<comment>null</comment>' : $resumeJobAlignmentCheckInputPath],
            ['Agent platform' => $agentPlatform]
        );

        $io->section('Agent');

        $agent = $this->resumeEditorLocator->getAgentByPlatform($normalizedAgentPlatform);
        $model = $this->resumeEditorLocator->getModelByPlatform($normalizedAgentPlatform);
        $modelParams = $this->resumeEditorLocator->getModelParamsByPlatform($normalizedAgentPlatform);

        $result = $this->callAgentAndStreamExecutionProgress(
            $io,
            $agent,
            $this->buildAgentInput($resume, $resumeFactCheck, $resumeJobAlignmentCheck),
            $model,
            $modelParams
        );

        $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeOutputPath), $result->thinking);
        $this->filesystem->dumpFile($resolvedResumeOutputPath, $this->sanitizeResultText($result->result));

        $io->success(\sprintf('Resume edit generated and written to %s.', $resumeOutputPath));

        return Command::SUCCESS;
    }

    private function fetchResume(string $resumeInputPath): string
    {
        $realResumeInputPath = $this->filesystem->realProjectPath($resumeInputPath);
        $trimmedResume = u($this->filesystem->readFile($realResumeInputPath))->trim();

        if ($trimmedResume->isEmpty()) {
            throw new \RuntimeException('Resume cannot be empty.');
        }

        return $trimmedResume->toString();
    }

    private function fetchResumeFactCheck(string $resumeFactCheckInputPath): string
    {
        $realResumeFactCheckInputPath = $this->filesystem->realProjectPath($resumeFactCheckInputPath);
        $trimmedResumeFactCheck = u($this->filesystem->readFile($realResumeFactCheckInputPath))->trim();

        if ($trimmedResumeFactCheck->isEmpty()) {
            throw new \RuntimeException('Resume fact-check cannot be empty.');
        }

        return $trimmedResumeFactCheck->toString();
    }

    private function fetchResumeJobAlignmentCheck(?string $resumeJobAlignmentCheckInputPath): ?string
    {
        $resumeJobAlignmentCheck = null;

        if (null !== $resumeJobAlignmentCheck) {
            $realResumeJobAlignmentCheckInputPath = $this->filesystem->realProjectPath($resumeJobAlignmentCheck);
            $trimmedResumeJobAlignmentCheck = u($this->filesystem->readFile($realResumeJobAlignmentCheckInputPath))->trim();

            if ($trimmedResumeJobAlignmentCheck->isEmpty()) {
                throw new \RuntimeException('Resume job-alignment-check cannot be empty.');
            }

            $resumeJobAlignmentCheck = $trimmedResumeJobAlignmentCheck->toString();
        }

        return $resumeJobAlignmentCheck;
    }

    private function buildAgentInput(string $resume, string $resumeFactCheck, ?string $resumeJobAlignmentCheck): string
    {
        $inputParts = ['Edit a complete, high-impact resume in Markdown format based on the following:'];

        $inputParts[] = \sprintf(
            <<<MD
            Edit this resume:

            <BASE_RESUME>
            %s
            </BASE_RESUME>

            Using this fact-check report:

            <FACT_CHECK_REPORT>
            %s
            </FACT_CHECK_REPORT>
            MD,
            $resume,
            $resumeFactCheck
        );

        if (null !== $resumeJobAlignmentCheck) {
            $inputParts[] = \sprintf(
                <<<MD
                And using this job-alignment-check report:

                <ALIGNMENT_REPORT>
                %s
                </ALIGNMENT_REPORT>
                MD,
                $resumeJobAlignmentCheck
            );
        }

        $inputParts[] = 'Output only the final Markdown resume. Do not include introductory text, explanations, or enclosing code block fences.';

        return implode("\n\n", $inputParts);
    }

    private function sanitizeResultText(string $resultText): string
    {
        $sanitizedResultText = u($resultText)->trim()->toString();

        if (preg_match('/^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$/s', $sanitizedResultText, $matches)) {
            $sanitizedResultText = u($matches[1])->trim()->toString();
        }

        return $sanitizedResultText;
    }
}
