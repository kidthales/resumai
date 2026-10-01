<?php

declare(strict_types=1);

namespace App\Command\Resume;

use App\AI\Agent\Toolbox\ArchetypeDirectoryTool;
use App\Console\Style\DefinitionListConverter;
use App\Filesystem\FilesystemV2;
use App\Service\ResumeDrafterLocator;
use Symfony\AI\Agent\Execution\Update\Progress;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
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
    name: 'app:resume:draft:v2',
    description: 'Draft a resume optionally tailored to a job description and candidate archetype',
)]
final readonly class DraftCommand
{
    public function __construct(
        private ResumeDrafterLocator $resumeDrafterLocator,
        private ArchetypeDirectoryTool $archetypeDirectoryTool,
        private FilesystemV2 $filesystem,
        private DefinitionListConverter $definitionListConverter,
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Resume output filepath')] string $resumeOutputPath,
        #[Argument('Path to a job description file')] ?string $jobDescriptionPath = null,
        #[Option('Archetype filename', 'archetype', 'a')] ?string $archetypeFilename = null,
        #[Option('Agent platform', 'platform', 'p')] string $platform = 'ollama',
    ): int {
        $io->title('Resume Draft');

        $io->section('Parameters');

        $resolvedResumeOutputPath = $this->filesystem->resolveProjectPath($resumeOutputPath);
        $jobDescription = $this->fetchJobDescription($jobDescriptionPath);
        $archetype = $this->fetchArchetype($archetypeFilename);
        $normalizedPlatform = u($platform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume output filepath' => $resumeOutputPath],
            ['Path to a job description file' => null === $jobDescriptionPath ? '<comment>null</comment>' : $jobDescriptionPath],
            ['Archetype filename' => null === $archetypeFilename ? '<comment>null</comment>' : $archetypeFilename],
            ['Agent platform' => $platform]
        );

        $agent = $this->resumeDrafterLocator->getAgentByPlatform($normalizedPlatform);
        $model = $this->resumeDrafterLocator->getModelByPlatform($normalizedPlatform);
        $modelParams = $this->resumeDrafterLocator->getModelParamsByPlatform($normalizedPlatform);

        $io->section(\sprintf('Agent: %s', $agent->getName()));

        $io->definitionList(
            $model,
            new TableSeparator(),
            ...$this->definitionListConverter->convert($modelParams),
        );

        $indicator = new ProgressIndicator($io);
        $indicator->start('Initializing...');

        $execution = $agent->call($this->buildAgentInput($jobDescription, $archetype), ['stream' => true, ...$modelParams]);

        $execution->onProgress(function (Progress $progress) use ($indicator) {
            $indicator->advance();
            $indicator->setMessage($progress->getMessage());
        });

        $resultText = '';
        $thinkingText = '';
        foreach ($execution->asStream() as $delta) {
            $indicator->advance();

            if ($delta instanceof ThinkingDelta) {
                $thinkingText .= $delta->getThinking();

                $preview = str_replace("\n", ' ', mb_substr($thinkingText, -40));
                $indicator->setMessage(sprintf('Thinking: "...%s"', $preview));
            } elseif ($delta instanceof TextDelta) {
                $resultText .= $delta->getText();

                $preview = str_replace("\n", ' ', mb_substr($resultText, -40));
                $indicator->setMessage(sprintf('Generating: "...%s"', $preview));
            }
        }

        $indicator->finish('<info>Execution completed.</info>');

        $this->filesystem->dumpFile(\sprintf('%s.thunk', $resolvedResumeOutputPath), $thinkingText);
        $this->filesystem->dumpFile($resolvedResumeOutputPath, $this->sanitizeResultText($resultText));

        $io->success(\sprintf('Resume draft generated and written to %s.', $resumeOutputPath));

        return Command::SUCCESS;
    }

    private function fetchJobDescription(?string $jobDescriptionPath): ?string
    {
        $jobDescription = null;

        if (null !== $jobDescriptionPath) {
            $realJobDescriptionPath = $this->filesystem->realProjectPath($jobDescriptionPath);
            $trimmedJobDescription = u($this->filesystem->readFile($realJobDescriptionPath))->trim();

            if ($trimmedJobDescription->isEmpty()) {
                throw new \RuntimeException('Job description cannot be empty.');
            }

            $jobDescription = $trimmedJobDescription->toString();
        }

        return $jobDescription;
    }

    private function fetchArchetype(?string $archetypeFilename): ?string
    {
        $archetype = null;

        if (null !== $archetypeFilename) {
            $trimmedArchetype = u($this->archetypeDirectoryTool->readArchetype($archetypeFilename))->trim();

            if ($trimmedArchetype->isEmpty()) {
                throw new \RuntimeException(\sprintf('Archetype "%s" cannot be empty.', $archetypeFilename));
            }

            $archetype = $trimmedArchetype->toString();
        }

        return $archetype;
    }

    private function buildAgentInput(?string $jobDescription, ?string $archetype): string
    {
        $inputParts = ['Synthesize a complete, high-impact resume in Markdown format based on the candidate profile.'];

        if (null !== $jobDescription) {
            $inputParts[] = \sprintf(
                <<<MD
                Target the resume specifically to the following Job Description:
                <job_description>
                %s
                </job_description>
                MD,
                $jobDescription
            );
        }

        if (null !== $archetype) {
            $inputParts[] = \sprintf(
                <<<MD
                Position the candidate according to the following Candidate Archetype:
                <candidate_archetype>
                %s
                </candidate_archetype>
                MD,
                $archetype
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
