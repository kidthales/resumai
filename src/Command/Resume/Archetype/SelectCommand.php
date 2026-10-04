<?php

declare(strict_types=1);

namespace App\Command\Resume\Archetype;

use App\Command\CallAgentAndStreamExecutionProgressTrait;
use App\Filesystem\Filesystem;
use App\Service\ResumeArchetypeSelectorLocator;
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
    name: 'app:resume:archetype:select',
    description: 'Select a suitable resume archetype for a given job description',
)]
final class SelectCommand
{
    use CallAgentAndStreamExecutionProgressTrait;

    public function __construct(
        private readonly ResumeArchetypeSelectorLocator $resumeArchetypeSelectorLocator,
        private readonly Filesystem $filesystem,
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Job description input filepath')] string $jobDescriptionInputPath,
        #[Option('Resume archetype selection output filepath (JSON)', 'output', 'o')] ?string $resumeArchetypeSelectionOutputPath = null,
        #[Option('Agent platform', 'platform', 'p')] string $agentPlatform = 'ollama',
    ): int {
        $io->title('Resume Archetype Select');

        $io->section('Parameters');

        $jobDescription = $this->fetchJobDescription($jobDescriptionInputPath);
        $resolvedResumeArchetypeSelectionOutputPath = null === $resumeArchetypeSelectionOutputPath ? null : $this->filesystem->resolveProjectPath($resumeArchetypeSelectionOutputPath);
        $normalizedAgentPlatform = u($agentPlatform)->trim()->lower()->toString();

        $io->definitionList(
            ['Job description input filepath' => $jobDescriptionInputPath],
            ['Resume archetype selection output filepath' => null === $resumeArchetypeSelectionOutputPath ? '<comment>null</comment>' : $resumeArchetypeSelectionOutputPath],
            ['Agent platform' => $agentPlatform]
        );

        $io->section('Agent');

        $agent = $this->resumeArchetypeSelectorLocator->getAgentByPlatform($normalizedAgentPlatform);
        $model = $this->resumeArchetypeSelectorLocator->getModelByPlatform($normalizedAgentPlatform);
        $modelParams = $this->resumeArchetypeSelectorLocator->getModelParamsByPlatform($normalizedAgentPlatform);

        $result = $this->callAgentAndStreamExecutionProgress(
            $io,
            $agent,
            $this->buildAgentInput($jobDescription),
            $model,
            $modelParams
        );

        $io->section('Result');

        $sanitizedResultText = $this->sanitizeResultText($result->result);
        $parsedResult = $this->parseResultText($sanitizedResultText);

        if ('' !== $result->thinking) {
            $io->writeln(\sprintf('<fg=gray>%s</fg=gray>', $result->thinking));
        }

        $io->outlineInfo(array_values($parsedResult));

        if (null === $resolvedResumeArchetypeSelectionOutputPath) {
            $io->success('Resume archetype selection generated.');
        } else {
            $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeArchetypeSelectionOutputPath), $result->thinking);
            $this->filesystem->dumpFile($resolvedResumeArchetypeSelectionOutputPath, $sanitizedResultText);

            $io->success(\sprintf('Resume archetype selection generated and written to %s.', $resumeArchetypeSelectionOutputPath));
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
            throw new \RuntimeException(\sprintf('Failed to parse resume_archetype_selector result as JSON. Result text: %s', $resultText), previous: $e);
        }

        if (!\is_array($archetypeSelection)) {
            throw new \RuntimeException(\sprintf('Invalid resume_archetype_selector result structure. Expected JSON object. Result text: %s', $resultText));
        }

        $archetypeFilename = $archetypeSelection['archetype_filename'] ?? $archetypeSelection['archetypeFilename'] ?? null;

        if (!\is_string($archetypeFilename) || '' === trim($archetypeFilename)) {
            throw new \RuntimeException(\sprintf('Missing or invalid "archetype_filename" in resume_archetype_selector result structure. Result text: %s', $resultText));
        }

        $archetypeRationale = $archetypeSelection['rationale'] ?? null;

        if (!\is_string($archetypeRationale) || '' === trim($archetypeRationale)) {
            throw new \RuntimeException(\sprintf('Missing or invalid "rationale" in resume_archetype_selector result structure. Result text: %s', $resultText));
        }

        return [
            'archetype_filename' => trim($archetypeFilename),
            'rationale' => trim($archetypeRationale),
        ];
    }
}
