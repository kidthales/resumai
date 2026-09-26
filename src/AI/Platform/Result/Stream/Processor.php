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

namespace App\AI\Platform\Result\Stream;

use App\AI\Agent\Execution\Processor as ExecutionProcessor;
use App\AI\Platform\Result\Processor as ResultProcessor;
use App\AI\Platform\Result\Stream\Exception\UnexpectedDeltaTypeException;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;

/**
 * Tier 1 stream processor for iterating and dispatching chunk deltas from a stream result.
 *
 * This service consumes a {@see StreamResult} generator, notifying callers of stream lifecycle
 * events (start and finish) and routing content deltas to specialized callbacks based on their type
 * ({@see TextDelta} or {@see ThinkingDelta}).
 *
 * Architectural Hierarchy:
 * - Tier 1: {@see Processor} (Stream delta processor - this service)
 * - Tier 2: {@see ResultProcessor} (Platform result dispatcher)
 * - Tier 3: {@see ExecutionProcessor} (Agent execution coordinator)
 *
 * @phpstan-type StreamResultCallback = callable(StreamResult): void
 * @phpstan-type TextDeltaCallback = callable(TextDelta): void
 * @phpstan-type ThinkingDeltaCallback = callable(ThinkingDelta): void
 *
 * @see ResultProcessor
 * @see ExecutionProcessor
 *
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class Processor
{
    /**
     * Iterates over a StreamResult generator, dispatching lifecycle events and content deltas.
     *
     * @example
     * ```php
     * $processor = new Processor();
     * $processor->process(
     *     $streamResult,
     *     onStreamResultStart: fn (StreamResult $stream) => $output->writeln('Stream started'),
     *     textDeltaProcessor: fn (TextDelta $delta) => $output->write($delta->getText()),
     *     thinkingDeltaProcessor: fn (ThinkingDelta $delta) => $logger->debug($delta->getThinking()),
     *     onStreamResultFinish: fn (StreamResult $stream) => $output->writeln('Stream finished'),
     * );
     * ```
     *
     * @param StreamResult                                              $result                 The stream result generator containing content deltas
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultStart    Callback invoked before iterating the stream
     * @param TextDeltaCallback|(callable(TextDelta):void)|null         $textDeltaProcessor     Callback invoked for each text chunk delta
     * @param ThinkingDeltaCallback|(callable(ThinkingDelta):void)|null $thinkingDeltaProcessor Callback invoked for each reasoning/thinking chunk delta
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultFinish   Callback invoked after the stream is fully consumed
     *
     * @throws UnexpectedDeltaTypeException When encountering an unsupported DeltaInterface implementation
     */
    public function process(
        StreamResult $result,
        ?callable $onStreamResultStart = null,
        ?callable $textDeltaProcessor = null,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $onStreamResultFinish = null,
    ): void {
        if (null !== $onStreamResultStart) {
            $onStreamResultStart($result);
        }

        foreach ($result->getContent() as $delta) {
            if ($delta instanceof TextDelta) {
                if (null !== $textDeltaProcessor) {
                    $textDeltaProcessor($delta);
                }
            } elseif ($delta instanceof ThinkingDelta) {
                if (null !== $thinkingDeltaProcessor) {
                    $thinkingDeltaProcessor($delta);
                }
            } else {
                throw UnexpectedDeltaTypeException::forDelta($delta);
            }
        }

        if (null !== $onStreamResultFinish) {
            $onStreamResultFinish($result);
        }
    }
}
