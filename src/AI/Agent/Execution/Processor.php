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

namespace App\AI\Agent\Execution;

use App\AI\Platform\Result\Processor as ResultProcessor;
use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use Symfony\AI\Agent\Execution\Execution;
use Symfony\AI\Agent\Execution\Update\Progress;
use Symfony\AI\Agent\Execution\Update\Result;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;
use Symfony\AI\Platform\Result\TextResult;

/**
 * Tier 3 agent execution processor for coordinating agent lifecycle hooks and platform result processing.
 *
 * This service sits at the agent execution level, binding {@see Progress} and {@see Result} lifecycle
 * listeners to the active {@see Execution} instance before delegating the underlying platform
 * {@see \Symfony\AI\Platform\Result\ResultInterface} to the Tier 2 {@see ResultProcessor}.
 *
 * Architectural Hierarchy:
 * - Tier 1: {@see StreamProcessor} (Stream delta processor)
 * - Tier 2: {@see ResultProcessor} (Platform result dispatcher)
 * - Tier 3: {@see Processor} (Agent execution coordinator - this service)
 *
 * @phpstan-type TextResultCallback = callable(TextResult): void
 * @phpstan-type StreamResultCallback = callable(StreamResult): void
 * @phpstan-type TextDeltaCallback = callable(TextDelta): void
 * @phpstan-type ThinkingDeltaCallback = callable(ThinkingDelta): void
 * @phpstan-type ProgressCallback = callable(Progress): void
 * @phpstan-type ResultCallback = callable(Result): void
 *
 * @see ResultProcessor
 * @see StreamProcessor
 *
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class Processor
{
    public function __construct(
        private ResultProcessor $resultProcessor,
    ) {
    }

    /**
     * Binds lifecycle listeners to an Execution and processes its platform result.
     *
     * @example
     * ```php
     * $processor->process(
     *     $execution,
     *     textResultProcessor: fn (TextResult $res) => $text .= (string) $res->getContent(),
     *     textDeltaProcessor: fn (TextDelta $delta) => $text .= $delta->getText(),
     *     thinkingDeltaProcessor: fn (ThinkingDelta $delta) => $thinking .= $delta->getThinking(),
     *     onProgress: fn (Progress $prog) => $logger->info($prog->getUpdate()),
     * );
     * ```
     *
     * @param Execution                                                 $execution              The agent execution instance
     * @param TextResultCallback|(callable(TextResult):void)|null       $textResultProcessor    Callback invoked when result is a synchronous TextResult
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultStart    Callback invoked before stream iteration begins
     * @param TextDeltaCallback|(callable(TextDelta):void)|null         $textDeltaProcessor     Callback invoked for each text chunk delta
     * @param ThinkingDeltaCallback|(callable(ThinkingDelta):void)|null $thinkingDeltaProcessor Callback invoked for each thinking chunk delta
     * @param StreamResultCallback|(callable(StreamResult):void)|null   $onStreamResultFinish   Callback invoked after stream iteration completes
     * @param ProgressCallback|(callable(Progress):void)|null           $onProgress             Callback invoked on agent progress lifecycle updates
     * @param ResultCallback|(callable(Result):void)|null               $onResult               Callback invoked on agent result lifecycle updates
     */
    public function process(
        Execution $execution,
        ?callable $textResultProcessor = null,
        ?callable $onStreamResultStart = null,
        ?callable $textDeltaProcessor = null,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $onStreamResultFinish = null,
        ?callable $onProgress = null,
        ?callable $onResult = null,
    ): void {
        if (null !== $onProgress) {
            $execution->onProgress($onProgress);
        }

        if (null !== $onResult) {
            $execution->onResult($onResult);
        }

        $this->resultProcessor->process(
            $execution->getResult(),
            $textResultProcessor,
            $onStreamResultStart,
            $textDeltaProcessor,
            $thinkingDeltaProcessor,
            $onStreamResultFinish,
        );
    }
}
