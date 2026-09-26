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
use Symfony\AI\Agent\Execution\Execution;
use Symfony\AI\Agent\Execution\Update\Progress;
use Symfony\AI\Agent\Execution\Update\Result;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;
use Symfony\AI\Platform\Result\TextResult;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class Processor
{
    public function __construct(
        private ResultProcessor $resultProcessor,
    ) {
    }

    /**
     * @param (callable(TextResult):void)|null    $textResultProcessor
     * @param (callable(StreamResult):void)|null  $onStreamResultStart
     * @param (callable(TextDelta):void)|null     $textDeltaProcessor
     * @param (callable(ThinkingDelta):void)|null $thinkingDeltaProcessor
     * @param (callable(StreamResult):void)|null  $onStreamResultFinish
     * @param (callable(Progress):void)|null      $onProgress
     * @param (callable(Result):void)|null        $onResult
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
