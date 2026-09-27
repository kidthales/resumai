<?php

namespace App\Command;

use App\Filesystem\PathChecker;
use Symfony\AI\Agent\AgentInterface;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Style\SymfonyStyle;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\DependencyInjection\Attribute\Target;
use Symfony\Component\Filesystem\Filesystem;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsCommand(
    name: 'app:archetype:select',
    description: 'Select a suitable archetype for a given job description',
)]
final readonly class ArchetypeSelectCommand
{
    public function __construct(
        #[Autowire('%kernel.project_dir%')] private string $projectPath,
        #[Target('archetype_selector')] private AgentInterface $agent,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Path to a job description file')] string $jobDescriptionPath,
    ): int {
        $jobDescriptionPathChecker = new PathChecker($jobDescriptionPath);

        if ('' === $jobDescriptionPathChecker->path()) {
            throw new \InvalidArgumentException('Job description path cannot be empty.');
        }

        if (
            !($realJobDescriptionPath = $jobDescriptionPathChecker->realpath())
            || !$jobDescriptionPathChecker->hasBasepath($this->projectPath, useRealpath: true)
        ) {
            throw new \InvalidArgumentException(\sprintf('Job description path "%s" was not found.', $jobDescriptionPath));
        }

        $trimmedJobDescription = u($this->filesystem->readFile($realJobDescriptionPath))->trim();

        if ($trimmedJobDescription->isEmpty()) {
            throw new \InvalidArgumentException('Job description cannot be empty.');
        }

        $input = \sprintf(
            <<<MD
            Analyze the following target job description, use the available tools to evaluate matching candidate
            archetypes from the directory, and select the best archetype:

            <job_description>
            %s
            </job_description>

            Provide your decision as a valid JSON object matching the required schema.
            MD,
            $trimmedJobDescription
        );

        $content = $this->agent->call($input)->getContent();

        // TODO: Validate the content as JSON and handle errors
        $io->writeln($content);

        return Command::SUCCESS;
    }
}
