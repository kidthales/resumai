<?php

declare(strict_types=1);

namespace App\Command\Resume;

use App\Command\CallAgentAndStreamExecutionProgressTrait;
use App\Console\Transformer\DefinitionListTransformer;
use App\Filesystem\Filesystem;
use App\Service\ResumeDrafterLocator;
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
    name: 'app:resume:draft',
    description: 'Draft a resume optionally tailored to a job description and candidate archetype',
)]
final readonly class DraftCommand
{
    use CallAgentAndStreamExecutionProgressTrait;

    public function __construct(
        private ResumeDrafterLocator $resumeDrafterLocator,
        private Filesystem $filesystem,
        private DefinitionListTransformer $definitionListTransformer,
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Resume output filepath')] string $resumeOutputPath,
        #[Option('Job description input filepath', 'job', 'j')] ?string $jobDescriptionInputPath = null,
        #[Option('Archetype input filename', 'archetype', 'a')] ?string $archetypeInputFilename = null,
        #[Option('Agent platform', 'platform', 'p')] string $agentPlatform = 'ollama',
    ): int {
        $io->title('Resume Draft');

        $io->section('Parameters');

        $resolvedResumeOutputPath = $this->filesystem->resolveProjectPath($resumeOutputPath);
        $jobDescription = $this->fetchJobDescription($jobDescriptionInputPath);
        $archetype = $this->fetchArchetype($archetypeInputFilename);
        $normalizedAgentPlatform = u($agentPlatform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume output filepath' => $resumeOutputPath],
            ['Job description input filepath' => null === $jobDescriptionInputPath ? '<comment>null</comment>' : $jobDescriptionInputPath],
            ['Archetype input filename' => null === $archetypeInputFilename ? '<comment>null</comment>' : $archetypeInputFilename],
            ['Agent platform' => $agentPlatform]
        );

        $io->section('Agent');

        $agent = $this->resumeDrafterLocator->getAgentByPlatform($normalizedAgentPlatform);
        $model = $this->resumeDrafterLocator->getModelByPlatform($normalizedAgentPlatform);
        $modelParams = $this->resumeDrafterLocator->getModelParamsByPlatform($normalizedAgentPlatform);

        self::callAgentAndStreamExecutionProgress(
            $agent,
            $this->buildAgentInput($jobDescription, $archetype),
            $model,
            $modelParams,
            $io,
            $this->definitionListTransformer,
            $resultText,
            $thinkingText
        );

        $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeOutputPath), $thinkingText);
        $this->filesystem->dumpFile($resolvedResumeOutputPath, $this->sanitizeResultText($resultText));

        $io->success(\sprintf('Resume draft generated and written to %s.', $resumeOutputPath));

        return Command::SUCCESS;
    }

    private function fetchJobDescription(?string $jobDescriptionInputPath): ?string
    {
        $jobDescription = null;

        if (null !== $jobDescriptionInputPath) {
            $realJobDescriptionInputPath = $this->filesystem->realProjectPath($jobDescriptionInputPath);
            $trimmedJobDescription = u($this->filesystem->readFile($realJobDescriptionInputPath))->trim();

            if ($trimmedJobDescription->isEmpty()) {
                throw new \RuntimeException('Job description cannot be empty.');
            }

            $jobDescription = $trimmedJobDescription->toString();
        }

        return $jobDescription;
    }

    private function fetchArchetype(?string $archetypeInputFilename): ?string
    {
        $archetype = null;

        if (null !== $archetypeInputFilename) {
            $trimmedArchetype = u($this->filesystem->readArchetypeFile($archetypeInputFilename))->trim();

            if ($trimmedArchetype->isEmpty()) {
                throw new \RuntimeException(\sprintf('Archetype "%s" cannot be empty.', $archetypeInputFilename));
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
