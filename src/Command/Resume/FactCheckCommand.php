<?php

declare(strict_types=1);

namespace App\Command\Resume;

use App\Command\CallAgentAndStreamExecutionProgressTrait;
use App\Console\Style\DefinitionListConverter;
use App\Filesystem\Filesystem;
use App\Service\ResumeFactCheckerLocator;
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
    name: 'app:resume:fact-check',
    description: 'Fact check a resume',
)]
final readonly class FactCheckCommand
{
    use CallAgentAndStreamExecutionProgressTrait;

    public function __construct(
        private ResumeFactCheckerLocator $resumeFactCheckerLocator,
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
        #[Argument('Resume input filepath')] string $resumeInputPath,
        #[Argument('Resume fact-check output filepath')] string $resumeFactCheckOutputPath,
        #[Option('Agent platform', 'platform', 'p')] string $agentPlatform = 'ollama',
    ): int {
        $io->title('Resume Fact-Check');

        $io->section('Parameters');

        $resume = $this->fetchResume($resumeInputPath);
        $resolvedResumeFactCheckOutputPath = $this->filesystem->resolveProjectPath($resumeFactCheckOutputPath);
        $normalizedAgentPlatform = u($agentPlatform)->trim()->lower()->toString();

        $io->definitionList(
            ['Resume input filepath' => $resumeInputPath],
            ['Resume fact-check output filepath' => $resumeFactCheckOutputPath],
            ['Agent platform' => $agentPlatform]
        );

        $io->section('Agent');

        $agent = $this->resumeFactCheckerLocator->getAgentByPlatform($normalizedAgentPlatform);
        $model = $this->resumeFactCheckerLocator->getModelByPlatform($normalizedAgentPlatform);
        $modelParams = $this->resumeFactCheckerLocator->getModelParamsByPlatform($normalizedAgentPlatform);

        self::callAgentAndStreamExecutionProgress(
            $agent,
            $this->buildAgentInput($resume),
            $model,
            $modelParams,
            $io,
            $this->definitionListConverter,
            $resultText,
            $thinkingText
        );

        $this->filesystem->dumpFile(\sprintf('%s.thonk', $resolvedResumeFactCheckOutputPath), $thinkingText);
        $this->filesystem->dumpFile($resolvedResumeFactCheckOutputPath, $this->sanitizeResultText($resultText));

        $io->success(\sprintf('Resume fact-check generated and written to %s.', $resumeFactCheckOutputPath));

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

    private function buildAgentInput(string $resume): string
    {
        return \sprintf(
            <<<MD
            Perform a fact-check and coherence analysis, against Verified Source Facts, for the following resume:

            <resume>
            %s
            </resume>

            Output only the final Markdown executive summary. Do not include introductory text, explanations, or enclosing code block fences.
            MD,
            $resume
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
