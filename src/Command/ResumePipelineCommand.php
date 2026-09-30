<?php

declare(strict_types=1);

namespace App\Command;

use App\Filesystem\PathChecker;
use Symfony\Bundle\FrameworkBundle\Console\Application;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Attribute\Option;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Input\ArrayInput;
use Symfony\Component\Console\Output\BufferedOutput;
use Symfony\Component\Console\Style\SymfonyStyle;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\Filesystem\Filesystem;
use Symfony\Component\Filesystem\Path;
use Symfony\Component\HttpKernel\KernelInterface;
use Symfony\Component\Uid\Uuid;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsCommand(
    name: 'app:resume:pipeline',
    description: 'Run a pipeline of agents to create a resume optionally tailored to a job description and candidate archetype',
)]
final readonly class ResumePipelineCommand
{
    private const string HISTORY_FILENAME = 'history.md';

    private const int MAX_PLATFORM_RETRIES = 3;

    public function __construct(
        #[Autowire('%kernel.project_dir%')] private string $projectPath,
        private KernelInterface $kernel,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

    /**
     * @throws \Symfony\Component\Console\Exception\ExceptionInterface
     * @throws \Throwable
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Path to pipeline output directory')] string $pipelineOutputPath,
        #[Argument('Path to a job description file')] ?string $jobDescriptionPath = null,
        #[Option('Archetype filename', 'archetype', 'a')] ?string $archetypeFilename = null,
        #[Option('Exclude archetypes', 'exclude-archetypes')] bool $excludeArchetypes = false,
    ): int {
        $pipelineOutputPathChecker = new PathChecker($pipelineOutputPath);

        if ('' === $pipelineOutputPathChecker->path()) {
            throw new \InvalidArgumentException('Resume output path cannot be empty.');
        }

        if (
            !($realPipelineOutputPath = $pipelineOutputPathChecker->realpath())
            || !$pipelineOutputPathChecker->hasBasepath($this->projectPath, useRealpath: true)
        ) {
            throw new \InvalidArgumentException(\sprintf('Resume output path "%s" was not found.', $pipelineOutputPath));
        }

        if (null !== $archetypeFilename && true === $excludeArchetypes) {
            throw new \InvalidArgumentException('Cannot specify both --archetype and --exclude-archetypes.');
        }

        $uid = Uuid::v7();
        $title = \sprintf('Resume Pipeline: %s', $uid->toString());
        $parameters = [
            null === $jobDescriptionPath ? 'null' : \sprintf('%s', $jobDescriptionPath),
            null === $archetypeFilename ? 'null' : \sprintf('%s', $archetypeFilename),
            $excludeArchetypes ? 'true' : 'false',
        ];

        $io->title($title);
        $io->horizontalTable(
            ['job-description-path', 'archetype', 'exclude-archetypes'],
            [
                [
                    null === $jobDescriptionPath ? \sprintf('<comment>%s</comment>', $parameters[0]) : $parameters[0],
                    null === $archetypeFilename ? \sprintf('<comment>%s</comment>', $parameters[1]) : $parameters[1],
                    \sprintf('<comment>%s</comment>', $parameters[2]),
                ],
            ]
        );

        $history = [];
        $history[] = \sprintf(
            <<<MD
            # %s

            **PARAMETERS**

            |job-description-path|archetype|exclude-archetypes|
            |---|---|---|
            |`%s`|`%s`|`%s`|

            MD,
            $title,
            ...$parameters
        );

        try {
            $application = new Application($this->kernel);
            $application->setAutoExit(false);

            $io->section('1. Archetype Select');
            $history[] = '## 1. Archetype Select';

            $platformRetries = 0;
            do {
                try {
                    $this->doArchetypeSelect(
                        $application,
                        $io,
                        $jobDescriptionPath,
                        $archetypeFilename,
                        $excludeArchetypes,
                        $history
                    );

                    $platformError = false;
                } catch (\Symfony\AI\Platform\Exception\RuntimeException $e) {
                    $platformError = true;

                    if (self::MAX_PLATFORM_RETRIES === $platformRetries++) {
                        throw $e;
                    }

                    $retryMessage = \sprintf('Retry attempt %d of %d...', $platformRetries, self::MAX_PLATFORM_RETRIES);

                    $io->outlineError($e->getMessage());
                    $io->writeln(\sprintf('Retry attempt %d of %d...', $platformRetries, self::MAX_PLATFORM_RETRIES));

                    $history[] = \sprintf(
                        <<<MD
                        **ERROR**

                        Message:

                        ```text
                        %s
                        ```

                        Trace:

                        ```text
                        %s
                        ```

                        %s
                        MD,
                        $e->getMessage(),
                        $e->getTraceAsString(),
                        $retryMessage
                    );
                } finally {
                    if (self::MAX_PLATFORM_RETRIES !== $platformRetries) {
                        $sleepDuration = 60 * (($platformRetries * ($platformError ? 1 : 0)) + 1);
                        $io->writeln(\sprintf('Sleeping for %d seconds...', $sleepDuration));
                        sleep($sleepDuration);
                    }
                }
            } while ($platformError);

            $io->section('2. Resume Draft');
            $history[] = '## 2. Resume Draft';
            $resumeDraftPath = $this->doResumeDraft(
                $application,
                $io,
                $realPipelineOutputPath,
                $jobDescriptionPath,
                $archetypeFilename,
                $uid,
                $history
            );

            $io->writeln('Sleeping for 60 seconds...');
            sleep(60);

            $io->section('3. Resume Smell Check');
            $history[] = '## 3. Resume Smell Check';
            $resumeSmellCheckPath = $this->doResumeSmellCheck(
                $application,
                $io,
                $realPipelineOutputPath,
                $resumeDraftPath,
                $uid,
                $history
            );

            $io->writeln('Sleeping for 60 seconds...');
            sleep(60);

            $io->section('4. Resume Revise');
            $history[] = '## 4. Resume Revise';
            $resumeRevisedPath = $this->doResumeRevise(
                $application,
                $io,
                $realPipelineOutputPath,
                $resumeDraftPath,
                $resumeSmellCheckPath,
                $uid,
                $history
            );
        } catch (\Throwable $e) {
            $history[] = \sprintf(
                <<<MD
                **ERROR**

                Message:

                ```text
                %s
                ```

                Trace:

                ```text
                %s
                ```
                MD,
                $e->getMessage(),
                $e->getTraceAsString()
            );

            throw $e;
        } finally {
            try {
                $historyPath = Path::join($realPipelineOutputPath, self::HISTORY_FILENAME);
                $this->filesystem->appendToFile($historyPath, \implode("\n\n", $history)."\n\n---\n\n");
            } catch (\Throwable $e) {
                $io->outlineError(\sprintf('Failed to append to history file: %s', $e->getMessage()));
                $io->writeln('Dumping history to console...');
                $io->writeln(\implode("\n\n", $history)."\n\n---\n");
            }
        }

        return Command::SUCCESS;
    }

    /**
     * @throws \Symfony\Component\Console\Exception\ExceptionInterface
     */
    private function doArchetypeSelect(
        Application $application,
        SymfonyStyle $io,
        ?string $jobDescriptionPath,
        ?string &$archetypeFilename,
        bool $excludeArchetypes,
        array &$history,
    ): void {
        if (null === $archetypeFilename && true !== $excludeArchetypes && null !== $jobDescriptionPath) {
            $command = $application->find('app:archetype:select');

            $input = new ArrayInput(['job-description-path' => $jobDescriptionPath]);
            $output = new BufferedOutput();

            $io->writeln('Executing: app:archetype:select...');

            $exitCode = $command->run($input, $output);

            if (Command::SUCCESS !== $exitCode) {
                throw new \RuntimeException('Command app:archetype:select failed.');
            }

            $trimmedOutput = u($output->fetch())->trim()->toString();

            if (preg_match('/^```(?:json)?\s*\n?(.*?)\n?```$/s', $trimmedOutput, $matches)) {
                $trimmedOutput = trim($matches[1]);
            } elseif (preg_match('/\{[\s\S]*\}/', $trimmedOutput, $matches)) {
                $trimmedOutput = $matches[0];
            }

            $history[] = \sprintf(
                <<<MD
                **ARCHETYPE SELECTOR AGENT**

                ```json
                %s
                ```
                MD,
                $trimmedOutput
            );

            try {
                $archetypeSelection = json_decode($trimmedOutput, true, \JSON_THROW_ON_ERROR);
            } catch (\JsonException $e) {
                throw new \RuntimeException('Failed to parse command app:archetype:select JSON output.', previous: $e);
            }

            if (!\is_array($archetypeSelection)) {
                throw new \RuntimeException('Invalid archetype selection response structure. Expected JSON object.');
            }

            $archetypeFilename = $archetypeSelection['archetype_filename'] ?? $archetypeSelection['archetypeFilename'] ?? null;

            if (!\is_string($archetypeFilename) || '' === trim($archetypeFilename)) {
                throw new \RuntimeException('Missing or invalid "archetype_id" in archetype response structure.');
            }

            $archetypeRationale = $archetypeSelection['rationale'] ?? null;

            if (!\is_string($archetypeRationale) || '' === trim($archetypeRationale)) {
                throw new \RuntimeException('Missing or invalid "rationale" in archetype response structure.');
            }

            $archetypeFilename = trim($archetypeFilename);
            $archetypeRationale = trim($archetypeRationale);

            $io->info(\sprintf('%s', $archetypeFilename));
            $io->comment(\sprintf('%s', $archetypeRationale));

            return;
        }

        if (null !== $archetypeFilename) {
            $io->info(\sprintf('%s', $archetypeFilename));
            $io->comment('User provided.');

            $history[] = \sprintf(
                <<<MD
                **USER PROVIDED**

                `%s`
                MD,
                $archetypeFilename
            );

            return;
        }

        $reason = $excludeArchetypes
            ? 'User excluded archetypes.'
            : 'User did not provide an archetype and job description.';

        $io->info('N/A');
        $io->comment(\sprintf('%s', $reason));

        $history[] = \sprintf(
            <<<MD
            **SKIPPED**

            %s
            MD,
            $reason
        );
    }

    /**
     * @throws \Symfony\Component\Console\Exception\ExceptionInterface
     */
    private function doResumeDraft(
        Application $application,
        SymfonyStyle $io,
        string $pipelineOutputPath,
        ?string $jobDescriptionPath,
        ?string $archetypeFilename,
        Uuid $uid,
        &$history,
    ): string {
        $command = $application->find('app:resume:draft');

        $parameters = ['job-description-path' => $jobDescriptionPath];

        if (null !== $archetypeFilename) {
            $parameters['--archetype'] = $archetypeFilename;
        }

        $input = new ArrayInput($parameters);
        $output = new BufferedOutput();

        $io->writeln('Executing: app:resume:draft...');

        $exitCode = $command->run($input, $output);

        if (Command::SUCCESS !== $exitCode) {
            throw new \RuntimeException('Command app:resume:draft failed.');
        }

        $trimmedOutput = u($output->fetch())->trim()->toString();

        if (preg_match('/^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$/s', $trimmedOutput, $matches)) {
            $trimmedOutput = trim($matches[1]);
        }

        $resumeDraftFilename = \sprintf('resume_draft.%s.md', $uid->toString());
        $resumeDraftPath = Path::join($pipelineOutputPath, $resumeDraftFilename);
        $this->filesystem->appendToFile($resumeDraftPath, $trimmedOutput);

        $io->info(\sprintf('%s', $resumeDraftFilename));
        $history[] = \sprintf(
            <<<MD
            **RESUME DRAFTED**

            `%s`
            MD,
            $resumeDraftFilename
        );

        return $resumeDraftPath;
    }

    /**
     * @throws \Symfony\Component\Console\Exception\ExceptionInterface
     */
    private function doResumeSmellCheck(
        Application $application,
        SymfonyStyle $io,
        string $pipelineOutputPath,
        string $resumePath,
        Uuid $uid,
        &$history,
    ): string {
        $command = $application->find('app:resume:smell-check');

        $input = new ArrayInput(['resume-path' => $resumePath]);
        $output = new BufferedOutput();

        $io->writeln('Executing: app:resume:smell-check...');

        $exitCode = $command->run($input, $output);

        if (Command::SUCCESS !== $exitCode) {
            throw new \RuntimeException('Command app:resume:smell-check failed.');
        }

        $trimmedOutput = u($output->fetch())->trim()->toString();

        if (preg_match('/^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$/s', $trimmedOutput, $matches)) {
            $trimmedOutput = trim($matches[1]);
        }

        $resumeSmellCheckFilename = \sprintf('resume_smell_check.%s.md', $uid->toString());
        $resumeSmellCheckPath = Path::join($pipelineOutputPath, $resumeSmellCheckFilename);
        $this->filesystem->appendToFile($resumeSmellCheckPath, $trimmedOutput);

        $io->info(\sprintf('%s', $resumeSmellCheckFilename));
        $history[] = \sprintf(
            <<<MD
            **RESUME SMELL CHECKED**

            `%s`
            MD,
            $resumeSmellCheckFilename
        );

        return $resumeSmellCheckPath;
    }

    /**
     * @throws \Symfony\Component\Console\Exception\ExceptionInterface
     */
    private function doResumeRevise(
        Application $application,
        SymfonyStyle $io,
        string $pipelineOutputPath,
        string $resumePath,
        string $resumeSmellCheckPath,
        Uuid $uid,
        &$history,
    ): string {
        $command = $application->find('app:resume:revise');

        $input = new ArrayInput(['resume-path' => $resumePath, 'resume-smell-check-path' => $resumeSmellCheckPath]);
        $output = new BufferedOutput();

        $io->writeln('Executing: app:resume:revise...');

        $exitCode = $command->run($input, $output);

        if (Command::SUCCESS !== $exitCode) {
            throw new \RuntimeException('Command app:resume:revise failed.');
        }

        $trimmedOutput = u($output->fetch())->trim()->toString();

        if (preg_match('/^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$/s', $trimmedOutput, $matches)) {
            $trimmedOutput = trim($matches[1]);
        }

        $resumeRevisedFilename = \sprintf('resume_revised.%s.md', $uid->toString());
        $resumeRevisedPath = Path::join($pipelineOutputPath, $resumeRevisedFilename);
        $this->filesystem->appendToFile($resumeRevisedPath, $trimmedOutput);

        $io->info(\sprintf('%s', $resumeRevisedFilename));
        $history[] = \sprintf(
            <<<MD
            **RESUME REVISED**

            `%s`
            MD,
            $resumeRevisedFilename
        );

        return $resumeRevisedPath;
    }
}
