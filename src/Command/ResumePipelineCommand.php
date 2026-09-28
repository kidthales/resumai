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

    public function __construct(
        #[Autowire('%kernel.project_dir%')] private string $projectPath,
        private KernelInterface $kernel,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

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
            ---

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

            if (null === $archetypeFilename && true !== $excludeArchetypes && null !== $jobDescriptionPath) {
                $archetypeSelectCommand = $application->find('app:archetype:select');
                $archetypeSelectCommandInput = new ArrayInput(['job-description-path' => $jobDescriptionPath]);
                $archetypeSelectCommandOutput = new BufferedOutput();

                $io->writeln('Executing: app:archetype:select...');
                $archetypeSelectCommandExitCode = $archetypeSelectCommand->run($archetypeSelectCommandInput, $archetypeSelectCommandOutput);

                if (Command::SUCCESS !== $archetypeSelectCommandExitCode) {
                    throw new \RuntimeException('Command app:archetype:select failed.');
                }

                $trimmedArchetypeSelection = u($archetypeSelectCommandOutput->fetch())->trim();

                $history[] = \sprintf(
                    <<<MD
                    **ARCHETYPE SELECTOR AGENT**

                    ```json
                    %s
                    ```
                    MD,
                    $trimmedArchetypeSelection->toString()
                );

                $archetypeSelection = json_decode($trimmedArchetypeSelection->toString(), true);
                $archetypeFilename = $archetypeSelection['archetype_filename'] ?? null;
                $archetypeRationale = $archetypeSelection['rationale'] ?? null;

                // TODO
                if (null === $archetypeFilename || null === $archetypeRationale) {
                    throw new \RuntimeException('Command app:archetype:select failed to return "archetype_filename" or "rationale".');
                }

                $io->info(\sprintf('%s', $archetypeFilename));
                $io->comment(\sprintf('%s', $archetypeRationale));
            } elseif (null !== $archetypeFilename) {
                $io->info(\sprintf('%s', $archetypeFilename));
                $io->comment('User provided.');
                $history[] = \sprintf(
                    <<<MD
                    **USER PROVIDED**

                    `%s`
                    MD,
                    $archetypeFilename
                );
            } else {
                $reason = $excludeArchetypes ? 'User excluded archetypes.' : 'User did not provide an archetype and job description.';
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

            $io->section('2. Resume Draft');
            $history[] = '## 2. Resume Draft';

            $resumeDraftCommand = $application->find('app:resume:draft');
            $resumeDraftCommandParameters = ['job-description-path' => $jobDescriptionPath];
            if (null !== $archetypeFilename) {
                $resumeDraftCommandParameters['--archetype'] = $archetypeFilename;
            }
            $resumeDraftCommandInput = new ArrayInput($resumeDraftCommandParameters);
            $resumeDraftCommandOutput = new BufferedOutput();

            $io->writeln('Executing: app:resume:draft...');
            $resumeDraftCommandExitCode = $resumeDraftCommand->run($resumeDraftCommandInput, $resumeDraftCommandOutput);

            if (Command::SUCCESS !== $resumeDraftCommandExitCode) {
                throw new \RuntimeException('Command app:resume:draft failed.');
            }

            $resumeDraftFilename = \sprintf('resume_draft.%s.md', $uid->toString());
            $resumeDraftPath = Path::join($realPipelineOutputPath, $resumeDraftFilename);
            $this->filesystem->appendToFile($resumeDraftPath, u($resumeDraftCommandOutput->fetch())->trim()->toString());

            $io->info(\sprintf('%s', $resumeDraftFilename));
            $history[] = \sprintf(
                <<<MD
                **RESUME DRAFTED**

                `%s`
                MD,
                $resumeDraftFilename
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
                $this->filesystem->appendToFile($historyPath, \implode("\n\n", $history)."\n\n");
            } catch (\Throwable $e) {
                $io->outlineError(\sprintf('Failed to append to history file: %s', $e->getMessage()));
                $io->info('Dumping history to console...');
                $io->write(\implode("\n\n", $history));
            }
        }

        return Command::SUCCESS;
    }
}
