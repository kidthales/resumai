<?php

/*
 * ResumAI
 * Copyright (C) 2026  Tristan Bonsor
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

declare(strict_types=1);

namespace App\Command;

use App\Console\Transformer\DefinitionListTransformerAwareTrait;
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
    use DefinitionListTransformerAwareTrait;

    /**
     * @throws \Symfony\AI\Agent\Exception\ExceptionInterface
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    protected function callAgentAndStreamExecutionProgress(
        SymfonyStyle $io,
        AgentInterface $agent,
        string $agentInput,
        string $model,
        array $modelParams,
    ): StreamedExecutionResult {
        $io->definitionList(
            $agent->getName(),
            new TableSeparator(),
            ['model' => $model],
            ...$this->definitionListTransformer->transform($modelParams),
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

        return new StreamedExecutionResult($resultText, $thinkingText, $messages);
    }
}
