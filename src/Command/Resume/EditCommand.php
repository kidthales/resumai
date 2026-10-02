<?php

declare(strict_types=1);

namespace App\Command\Resume;

use App\Command\StreamExecutionProgressTrait;
use App\Console\Style\DefinitionListConverter;
use App\Filesystem\FilesystemV2;
use App\Service\ResumeEditorLocator;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Attribute\Option;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Helper\ProgressIndicator;
use Symfony\Component\Console\Helper\TableSeparator;
use Symfony\Component\Console\Style\SymfonyStyle;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsCommand(
    name: 'app:resume:edit',
    description: 'Edit a resume with a fact-check report and an optional job-alignment report',
)]
final readonly class EditCommand
{
    use StreamExecutionProgressTrait;

    public function __construct(
        private ResumeEditorLocator $resumeEditorLocator,
        private FilesystemV2 $filesystem,
        private DefinitionListConverter $definitionListConverter,
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
        #[Option('Agent platform', 'platform', 'p')] string $platform = 'ollama',
    ): int {
        $io->title('Resume Edit');

        $io->section('Parameters');

        $resume = $this->fetchResume($resumeInputPath);
        $resumeFactCheck = $this->fetchResumeFactCheck($resumeFactCheckInputPath);
        $resolvedResumeOutputPath = $this->filesystem->resolveProjectPath($resumeOutputPath);
        $resumeJobAlignmentCheck = $this->fetchResumeJobAlignmentCheck($resumeJobAlignmentCheckInputPath);
        $normalizedPlatform = u($platform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume input filepath' => $resumeInputPath],
            ['Resume fact-check input filepath' => $resumeFactCheckInputPath],
            ['Resume output filepath' => $resumeOutputPath],
            ['Resume job-alignment-check input filepath' => null === $resumeJobAlignmentCheckInputPath ? '<comment>null</comment>' : $resumeJobAlignmentCheckInputPath],
            ['Agent platform' => $platform]
        );

        $io->section('Agent');

        $agent = $this->resumeEditorLocator->getAgentByPlatform($normalizedPlatform);
        $model = $this->resumeEditorLocator->getModelByPlatform($normalizedPlatform);
        $modelParams = $this->resumeEditorLocator->getModelParamsByPlatform($normalizedPlatform);

        $io->definitionList(
            $agent->getName(),
            new TableSeparator(),
            ['model' => $model],
            ...$this->definitionListConverter->convert($modelParams),
        );

        $indicator = new ProgressIndicator($io);
        $indicator->start('Initializing...');

        $execution = $agent->call($this->buildAgentInput($resume, $resumeFactCheck, $resumeJobAlignmentCheck), ['stream' => true, ...$modelParams]);

        $resultText = '';
        $thinkingText = '';
        $this->streamExecutionProgress($execution, $indicator, $resultText, $thinkingText);

        $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeOutputPath), $thinkingText);
        $this->filesystem->dumpFile($resolvedResumeOutputPath, $this->sanitizeResultText($resultText));

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
