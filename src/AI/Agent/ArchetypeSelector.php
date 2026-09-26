<?php

/*
 * This file is part of the ResumAI package.
 *
 * (c) Tristan Bonsor <kidthales@agogpixel.com>
 *
 * For the full copyright and license information, please view the LICENSE
 * file that was distributed with this source code.
 */

declare(strict_types=1);

namespace App\AI\Agent;

use App\AI\Agent\Execution\Processor;
use App\AI\Tool\ArchetypeDirectoryTool;
use Symfony\AI\Agent\AgentInterface;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\TextResult;
use Symfony\Component\DependencyInjection\Attribute\Target;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class ArchetypeSelector implements ArchetypeSelectorInterface
{
    public function __construct(
        #[Target('archetype_selector')]
        private AgentInterface $agent,
        private Processor $executionProcessor,
        private ArchetypeDirectoryTool $directoryTool,
    ) {
    }

    public function select(
        string $jobDescription,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $textDeltaProcessor = null,
    ): ArchetypeSelection {
        $trimmedJobDescription = trim($jobDescription);
        if ('' === $trimmedJobDescription) {
            throw new \InvalidArgumentException('Job description cannot be empty.');
        }

        $prompt = $this->buildPrompt($trimmedJobDescription);
        $execution = $this->agent->call($prompt);

        $responseText = '';
        $thinkingText = '';

        $this->executionProcessor->process(
            $execution,
            textResultProcessor: static function (TextResult $result) use (&$responseText): void {
                $responseText .= (string) $result->getContent();
            },
            textDeltaProcessor: static function (TextDelta $delta) use (&$responseText, $textDeltaProcessor): void {
                $responseText .= $delta->getText();
                if (null !== $textDeltaProcessor) {
                    $textDeltaProcessor($delta);
                }
            },
            thinkingDeltaProcessor: static function (ThinkingDelta $delta) use (&$thinkingText, $thinkingDeltaProcessor): void {
                $thinkingText .= $delta->getThinking();
                if (null !== $thinkingDeltaProcessor) {
                    $thinkingDeltaProcessor($delta);
                }
            },
        );

        $parsed = $this->parseResponse($responseText);
        $content = $this->directoryTool->readArchetype($parsed['archetype_id']);
        $archetypeName = $parsed['archetype_name'] ?? $this->formatTitleFromId($parsed['archetype_id']);

        return new ArchetypeSelection(
            archetypeId: $parsed['archetype_id'],
            archetypeName: $archetypeName,
            content: $content,
            rationale: $parsed['rationale'],
        );
    }

    private function buildPrompt(string $jobDescription): string
    {
        return \sprintf(
            "Analyze the following target job description, use the available tools to evaluate matching candidate archetypes from the directory, and select the best archetype:\n\n<job_description>\n%s\n</job_description>\n\nProvide your decision as a valid JSON object matching the required schema.",
            $jobDescription,
        );
    }

    /**
     * @return array{archetype_id: string, archetype_name: ?string, rationale: ?string}
     */
    private function parseResponse(string $output): array
    {
        $trimmed = trim($output);

        if (preg_match('/^```(?:json)?\s*\n?(.*?)\n?```$/s', $trimmed, $matches)) {
            $trimmed = trim($matches[1]);
        } elseif (preg_match('/\{[\s\S]*\}/', $trimmed, $matches)) {
            $trimmed = $matches[0];
        }

        try {
            $data = json_decode($trimmed, true, 512, \JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new \RuntimeException(\sprintf('Failed to parse archetype selection JSON response: %s. Raw response was: "%s"', $e->getMessage(), $output), 0, $e);
        }

        if (!\is_array($data)) {
            throw new \RuntimeException(\sprintf('Invalid archetype selection response structure. Expected JSON object, got: "%s"', $output));
        }

        $archetypeId = $data['archetype_id'] ?? $data['archetypeId'] ?? null;
        if (!\is_string($archetypeId) || '' === trim($archetypeId)) {
            throw new \RuntimeException(\sprintf('Missing or invalid "archetype_id" in agent response: "%s"', $output));
        }

        $archetypeName = $data['archetype_name'] ?? $data['archetypeName'] ?? null;
        $rationale = $data['rationale'] ?? null;

        return [
            'archetype_id' => trim($archetypeId),
            'archetype_name' => \is_string($archetypeName) && '' !== trim($archetypeName) ? trim($archetypeName) : null,
            'rationale' => \is_string($rationale) && '' !== trim($rationale) ? trim($rationale) : null,
        ];
    }

    private function formatTitleFromId(string $id): string
    {
        return ucwords(str_replace(['_', '-'], ' ', $id));
    }
}
