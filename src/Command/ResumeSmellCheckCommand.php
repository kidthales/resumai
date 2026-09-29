<?php

declare(strict_types=1);

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
    name: 'app:resume:smell-check',
    description: 'Smell check a resume',
)]
final readonly class ResumeSmellCheckCommand
{
    public function __construct(
        #[Autowire('%kernel.project_dir%')] private string $projectPath,
        #[Target('resume_smell_checker')] private AgentInterface $agent,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     */
    public function __invoke(
        SymfonyStyle $io,
        #[Argument('Path to a resume file')] string $resumePath,
    ): int {
        $resumePathChecker = new PathChecker($resumePath);

        if ('' === $resumePathChecker->path()) {
            throw new \InvalidArgumentException('Resume path cannot be empty.');
        }

        if (
            !($realResumePath = $resumePathChecker->realpath())
            || !$resumePathChecker->hasBasepath($this->projectPath, useRealpath: true)
        ) {
            throw new \InvalidArgumentException(\sprintf('Resume path "%s" was not found.', $resumePath));
        }

        $trimmedResume = u($this->filesystem->readFile($realResumePath))->trim();

        if ($trimmedResume->isEmpty()) {
            throw new \InvalidArgumentException('Resume cannot be empty.');
        }

        $input = \sprintf(
            <<<MD
            Perform a fact-check and coherence analysis, against Verified Source Facts, for the following resume:

            <resume>
            %s
            </resume>

            Output only the final Markdown executive summary. Do not include introductory text, explanations, or enclosing code block fences.
            MD,
            $trimmedResume
        );

        $content = $this->agent->call($input)->getContent();

        $io->writeln($content);

        return Command::SUCCESS;
    }
}
