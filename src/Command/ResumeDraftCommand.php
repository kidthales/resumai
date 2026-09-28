<?php

namespace App\Command;

use App\AI\Agent\Toolbox\ArchetypeDirectoryTool;
use App\Filesystem\PathChecker;
use Symfony\AI\Agent\AgentInterface;
use Symfony\Component\Console\Attribute\Argument;
use Symfony\Component\Console\Attribute\AsCommand;
use Symfony\Component\Console\Attribute\Option;
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
    name: 'app:resume:draft',
    description: 'Draft a resume optionally tailored to a job description and candidate archetype',
)]
final readonly class ResumeDraftCommand
{
    public function __construct(
        #[Autowire('%kernel.project_dir%')] private string $projectPath,
        #[Target('resume_drafter')] private AgentInterface $agent,
        private ArchetypeDirectoryTool $archetypeDirectoryTool,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Path to a job description file')] ?string $jobDescriptionPath = null,
        #[Option('Archetype filename', 'archetype', 'a')] ?string $archetypeFilename = null,
    ): int {
        $trimmedJobDescription = null;
        $trimmedArchetypeContent = null;

        if (null !== $jobDescriptionPath) {
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
        }

        if (null !== $archetypeFilename) {
            $trimmedArchetypeContent = u($this->archetypeDirectoryTool->readArchetype($archetypeFilename))->trim();

            if ($trimmedArchetypeContent->isEmpty()) {
                throw new \InvalidArgumentException(\sprintf('Archetype "%s" content cannot be empty.', $archetypeFilename));
            }
        }

        $inputParts = ['Synthesize a complete, high-impact resume in Markdown format based on the candidate profile.'];

        if (null !== $trimmedJobDescription) {
            $inputParts[] = \sprintf(
                <<<MD
                Target the resume specifically to the following Job Description:
                <job_description>
                %s
                </job_description>
                MD,
                $trimmedJobDescription
            );
        }

        if (null !== $trimmedArchetypeContent) {
            $inputParts[] = \sprintf(
                <<<MD
                Position the candidate according to the following Candidate Archetype:
                <candidate_archetype>
                %s
                </candidate_archetype>
                MD,
                $trimmedArchetypeContent
            );
        }

        $inputParts[] = 'Output only the final Markdown resume. Do not include introductory text, explanations, or enclosing code block fences.';

        $content = $this->agent->call(implode("\n\n", $inputParts))->getContent();

        // TODO: Validate the content as Markdown and handle errors
        $io->writeln($content);

        return Command::SUCCESS;
    }
}
