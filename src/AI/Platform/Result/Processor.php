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

namespace App\AI\Platform\Result;

use App\AI\Agent\Execution\Processor as ExecutionProcessor;
use App\AI\Platform\Result\Exception\UnsupportedResultTypeException;
use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use Symfony\AI\Platform\Result\ResultInterface;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;
use Symfony\AI\Platform\Result\TextResult;

/**
 * Tier 2 platform result processor for routing synchronous and streaming result interfaces.
 *
 * This service acts as the platform result dispatcher. It directly handles synchronous
 * {@see TextResult} instances and delegates {@see StreamResult} streams to the Tier 1
 * {@see StreamProcessor}.
 *
 * Architectural Hierarchy:
 * - Tier 1: {@see StreamProcessor} (Stream delta processor)
 * - Tier 2: {@see Processor} (Platform result dispatcher - this service)
 * - Tier 3: {@see ExecutionProcessor} (Agent execution coordinator)
 *
 * @phpstan-type TextResultCallback = callable(TextResult): void
 * @phpstan-type StreamResultCallback = callable(StreamResult): void
 * @phpstan-type TextDeltaCallback = callable(TextDelta): void
 * @phpstan-type ThinkingDeltaCallback = callable(ThinkingDelta): void
 *
 * @see StreamProcessor
 * @see ExecutionProcessor
 *
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class Processor
{
    public function __construct(private StreamProcessor $streamProcessor)
    {
    }

    /**
     * Processes a platform ResultInterface, dispatching synchronous text or streaming deltas.
     *
     * @example
     * ```php
     * $processor = new Processor(new StreamProcessor());
     * $processor->process(
     *     $result,
     *     textResultProcessor: fn (TextResult $res) => $output->write((string) $res->getContent()),
     *     textDeltaProcessor: fn (TextDelta $delta) => $output->write($delta->getText()),
     *     thinkingDeltaProcessor: fn (ThinkingDelta $delta) => $logger->debug($delta->getThinking()),
     * );
     * ```
     *
     * @param ResultInterface                                           $result                 The platform result to process (TextResult or StreamResult)
     * @param TextResultCallback|(callable(TextResult):void)|null       $textResultProcessor    Callback invoked when result is a synchronous TextResult
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultStart    Callback invoked before stream iteration begins
     * @param TextDeltaCallback|(callable(TextDelta):void)|null         $textDeltaProcessor     Callback invoked for each text chunk delta in a stream
     * @param ThinkingDeltaCallback|(callable(ThinkingDelta):void)|null $thinkingDeltaProcessor Callback invoked for each thinking chunk delta in a stream
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultFinish   Callback invoked after stream iteration completes
     *
     * @throws UnsupportedResultTypeException When encountering an unsupported ResultInterface implementation
     */
    public function process(
        ResultInterface $result,
        ?callable $textResultProcessor = null,
        ?callable $onStreamResultStart = null,
        ?callable $textDeltaProcessor = null,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $onStreamResultFinish = null,
    ): void {
        if ($result instanceof TextResult) {
            if (null !== $textResultProcessor) {
                $textResultProcessor($result);
            }
        } elseif ($result instanceof StreamResult) {
            $this->streamProcessor->process(
                $result,
                $onStreamResultStart,
                $textDeltaProcessor,
                $thinkingDeltaProcessor,
                $onStreamResultFinish
            );
        } else {
            throw UnsupportedResultTypeException::forResult($result);
        }
    }
}
