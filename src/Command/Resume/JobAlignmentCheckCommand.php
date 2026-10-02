<?php

declare(strict_types=1);

namespace App\Command\Resume;

use App\Command\CallAgentAndStreamExecutionProgressTrait;
use App\Console\Style\DefinitionListTransformer;
use App\Filesystem\Filesystem;
use App\Service\ResumeJobAlignmentCheckerLocator;
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
    name: 'app:resume:job-alignment-check',
    description: 'Check a resume for job alignment',
)]
final readonly class JobAlignmentCheckCommand
{
    use CallAgentAndStreamExecutionProgressTrait;

    public function __construct(
        private ResumeJobAlignmentCheckerLocator $resumeJobAlignmentCheckerLocator,
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
        #[Argument('Resume input filepath')] string $resumeInputPath,
        #[Argument('Job description input filepath')] string $jobDescriptionInputPath,
        #[Argument('Resume job-alignment-check output filepath')] string $resumeJobAlignmentCheckOutputPath,
        #[Option('Agent platform', 'platform', 'p')] string $agentPlatform = 'ollama',
    ): int {
        $io->title('Resume Job-Alignment-Check');

        $io->section('Parameters');

        $resume = $this->fetchResume($resumeInputPath);
        $jobDescription = $this->fetchJobDescription($jobDescriptionInputPath);
        $resolvedResumeJobAlignmentCheckOutputPath = $this->filesystem->resolveProjectPath($resumeJobAlignmentCheckOutputPath);
        $normalizedAgentPlatform = u($agentPlatform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume input filepath' => $resumeInputPath],
            ['Job description input filepath' => $jobDescriptionInputPath],
            ['Resume job-alignment-check output filepath' => $resumeJobAlignmentCheckOutputPath],
            ['Agent platform' => $agentPlatform]
        );

        $io->section('Agent');

        $agent = $this->resumeJobAlignmentCheckerLocator->getAgentByPlatform($normalizedAgentPlatform);
        $model = $this->resumeJobAlignmentCheckerLocator->getModelByPlatform($normalizedAgentPlatform);
        $modelParams = $this->resumeJobAlignmentCheckerLocator->getModelParamsByPlatform($normalizedAgentPlatform);

        self::callAgentAndStreamExecutionProgress(
            $agent,
            $this->buildAgentInput($resume, $jobDescription),
            $model,
            $modelParams,
            $io,
            $this->definitionListTransformer,
            $resultText,
            $thinkingText
        );

        $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeJobAlignmentCheckOutputPath), $thinkingText);
        $this->filesystem->dumpFile($resolvedResumeJobAlignmentCheckOutputPath, $this->sanitizeResultText($resultText));

        $io->success(\sprintf('Resume job-alignment-check generated and written to %s.', $resumeJobAlignmentCheckOutputPath));

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

    private function fetchJobDescription(string $jobDescriptionInputPath): string
    {
        $realJobDescriptionInputPath = $this->filesystem->realProjectPath($jobDescriptionInputPath);
        $trimmedJobDescription = u($this->filesystem->readFile($realJobDescriptionInputPath))->trim();

        if ($trimmedJobDescription->isEmpty()) {
            throw new \RuntimeException('Job description cannot be empty.');
        }

        return $trimmedJobDescription->toString();
    }

    private function buildAgentInput(string $resume, string $jobDescription): string
    {
        return \sprintf(
            <<<MD
            Perform a job-alignment-check and analysis for the following candidate resume and job description:

            <candidate_resume>
            %s
            </candidate_resume>

            <job_description>
            %s
            </job_description>

            Output only the final Markdown executive summary. Do not include introductory text, explanations, or enclosing code block fences.
            MD,
            $resume,
            $jobDescription,
        );
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
