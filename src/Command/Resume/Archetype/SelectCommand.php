<?php

namespace App\Command\Resume\Archetype;

use App\Command\StreamExecutionProgressTrait;
use App\Console\Style\DefinitionListConverter;
use App\Filesystem\Filesystem;
use App\Service\ResumeArchetypeSelectorLocator;
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
    name: 'app:resume:archetype:select',
    description: 'Select a suitable resume archetype for a given job description',
)]
final readonly class SelectCommand
{
    use StreamExecutionProgressTrait;

    public function __construct(
        private ResumeArchetypeSelectorLocator $resumeArchetypeSelectorLocator,
        private Filesystem $filesystem,
        private DefinitionListConverter $definitionListConverter,
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Path to a job description input file')] string $jobDescriptionInputPath,
        #[Option('Resume archetype selection output filepath', 'output', 'o')] ?string $resumeArchetypeSelectionOutputPath = null,
        #[Option('Agent platform', 'platform', 'p')] string $platform = 'ollama',
    ): int {
        $io->title('Resume Archetype Selection');

        $io->section('Parameters');

        $jobDescription = $this->fetchJobDescription($jobDescriptionInputPath);
        $resolvedResumeArchetypeSelectionOutputPath = null === $resumeArchetypeSelectionOutputPath ? null : $this->filesystem->resolveProjectPath($resumeArchetypeSelectionOutputPath);
        $normalizedPlatform = u($platform)->trim()->lower()->toString();

        $io->definitionList(
            ['Path to a job description input file' => $jobDescriptionInputPath],
            ['Resume archetype selection output filepath' => null === $resumeArchetypeSelectionOutputPath ? '<comment>null</comment>' : $resumeArchetypeSelectionOutputPath],
            ['Agent platform' => $platform]
        );

        $io->section('Agent');

        $agent = $this->resumeArchetypeSelectorLocator->getAgentByPlatform($normalizedPlatform);
        $model = $this->resumeArchetypeSelectorLocator->getModelByPlatform($normalizedPlatform);
        $modelParams = $this->resumeArchetypeSelectorLocator->getModelParamsByPlatform($normalizedPlatform);

        $io->definitionList(
            $agent->getName(),
            new TableSeparator(),
            ['model' => $model],
            ...$this->definitionListConverter->convert($modelParams),
        );

        $indicator = new ProgressIndicator($io);
        $indicator->start('Initializing...');

        $execution = $agent->call($this->buildAgentInput($jobDescription), ['stream' => true, ...$modelParams]);

        $resultText = '';
        $thinkingText = '';
        $this->streamExecutionProgress($execution, $indicator, $resultText, $thinkingText);

        $io->section('Result');

        $sanitizedResultText = $this->sanitizeResultText($resultText);
        $parsedResult = $this->parseResultText($sanitizedResultText);

        $io->writeln(\sprintf("<fg=gray><thinking>\n%s\n</thinking></fg=gray>", '' === $thinkingText ? 'n/a' : $thinkingText));
        $io->outlineInfo(array_values($parsedResult));

        if (null === $resolvedResumeArchetypeSelectionOutputPath) {
            $io->success('Resume archetype selection generated.');
        } else {
            $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeArchetypeSelectionOutputPath), $thinkingText);
            $this->filesystem->dumpFile($resolvedResumeArchetypeSelectionOutputPath, $sanitizedResultText);

            $io->success(\sprintf('Resume archetype selection generated and written to %s.', $resolvedResumeArchetypeSelectionOutputPath));
        }

        return Command::SUCCESS;
    }

    private function fetchJobDescription(string $jobDescriptionInputPath): ?string
    {
        $realJobDescriptionInputPath = $this->filesystem->realProjectPath($jobDescriptionInputPath);
        $trimmedJobDescription = u($this->filesystem->readFile($realJobDescriptionInputPath))->trim();

        if ($trimmedJobDescription->isEmpty()) {
            throw new \RuntimeException('Job description cannot be empty.');
        }

        return $trimmedJobDescription->toString();
    }

    private function buildAgentInput(string $jobDescription): string
    {
        return \sprintf(
            <<<MD
            Analyze the following target job description, use the available tools to evaluate matching candidate
            archetypes from the directory, and select the best archetype:

            <job_description>
            %s
            </job_description>

            Provide your decision as a valid JSON object matching the required schema.
            MD,
            $jobDescription
        );
    }

    private function sanitizeResultText(string $resultText): string
    {
        $sanitizedResultText = u($resultText)->trim()->toString();

        if (preg_match('/^```(?:json)?\s*\n?(.*?)\n?```$/s', $sanitizedResultText, $matches)) {
            $sanitizedResultText = trim($matches[1]);
        } elseif (preg_match('/\{[\s\S]*\}/', $sanitizedResultText, $matches)) {
            $sanitizedResultText = $matches[0];
        }

        return $sanitizedResultText;
    }

    /**
     * @return array{
     *   'archetype_filename': string,
     *   'rationale': string,
     * }
     */
    private function parseResultText(string $resultText): array
    {
        try {
            $archetypeSelection = json_decode($resultText, true, \JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new \RuntimeException('Failed to parse resume_archetype_selector result as JSON.', previous: $e);
        }

        if (!\is_array($archetypeSelection)) {
            throw new \RuntimeException('Invalid resume_archetype_selector result structure. Expected JSON object.');
        }

        $archetypeFilename = $archetypeSelection['archetype_filename'] ?? $archetypeSelection['archetypeFilename'] ?? null;

        if (!\is_string($archetypeFilename) || '' === trim($archetypeFilename)) {
            throw new \RuntimeException('Missing or invalid "archetype_filename" in resume_archetype_selector result structure.');
        }

        $archetypeRationale = $archetypeSelection['rationale'] ?? null;

        if (!\is_string($archetypeRationale) || '' === trim($archetypeRationale)) {
            throw new \RuntimeException('Missing or invalid "rationale" in resume_archetype_selector result structure.');
        }

        return [
            'archetype_filename' => trim($archetypeFilename),
            'rationale' => trim($archetypeRationale),
        ];
    }
}
