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

use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use Symfony\AI\Platform\Result\ResultInterface;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;
use Symfony\AI\Platform\Result\TextResult;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class Processor
{
    public function __construct(private StreamProcessor $streamProcessor)
    {
    }

    /**
     * @param (callable(TextResult):void)|null    $textResultProcessor
     * @param (callable(StreamResult):void)|null  $onStreamResultStart
     * @param (callable(TextDelta):void)|null     $textDeltaProcessor
     * @param (callable(ThinkingDelta):void)|null $thinkingDeltaProcessor
     * @param (callable(StreamResult):void)|null  $onStreamResultFinish
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
            if (is_callable($textResultProcessor)) {
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
            throw new \InvalidArgumentException('Unexpected result type');
        }
    }
}
