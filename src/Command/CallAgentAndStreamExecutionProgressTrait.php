<?php

declare(strict_types=1);

namespace App\Command;

use App\Console\Style\DefinitionListConverter;
use Symfony\AI\Agent\AgentInterface;
use Symfony\AI\Agent\Execution\Update\Progress;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\Component\Console\Helper\ProgressIndicator;
use Symfony\Component\Console\Helper\TableSeparator;
use Symfony\Component\Console\Style\SymfonyStyle;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
trait CallAgentAndStreamExecutionProgressTrait
{
    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    protected static function callAgentAndStreamExecutionProgress(
        AgentInterface $agent,
        string $agentInput,
        string $model,
        array $modelParams,
        SymfonyStyle $io,
        DefinitionListConverter $definitionListConverter,
        ?string &$resultText = '',
        ?string &$thinkingText = '',
        ?array &$messages = [],
    ): void {
        $io->definitionList(
            $agent->getName(),
            new TableSeparator(),
            ['model' => $model],
            ...$definitionListConverter->convert($modelParams),
        );

        $indicator = new ProgressIndicator($io);
        $indicator->start('Initializing...');

        $execution = $agent->call($agentInput, [...$modelParams, 'stream' => true]);

        $execution->onProgress(function (Progress $progress) use ($indicator, &$messages) {
            $messages[] = $progress->getMessage();

            $indicator->advance();
            $indicator->setMessage($messages[count($messages) - 1]);
        });

        $resultText = '';
        $thinkingText = '';
        foreach ($execution->asStream() as $delta) {
            $indicator->advance();

            if ($delta instanceof ThinkingDelta) {
                $thinkingText .= $delta->getThinking();

                $preview = str_replace("\n", ' ', mb_substr($thinkingText, -40));
                $indicator->setMessage(sprintf('Thinking: "...%s"', $preview));
            } elseif ($delta instanceof TextDelta) {
                $resultText .= $delta->getText();

                $preview = str_replace("\n", ' ', mb_substr($resultText, -40));
                $indicator->setMessage(sprintf('Generating: "...%s"', $preview));
            }
        }

        $indicator->finish('<info>Execution completed.</info>');
    }
}
