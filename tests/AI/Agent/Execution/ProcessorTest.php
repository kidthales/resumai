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

namespace App\Tests\AI\Agent\Execution;

use App\AI\Agent\Execution\Processor;
use App\AI\Platform\Result\Processor as ResultProcessor;
use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
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
#[Group('ai')]
#[CoversClass(Processor::class)]
final class ProcessorTest extends TestCase
{
    #[Test]
    public function itProcessesExecutionWithNoCallbacks(): void
    {
        $callCount = 0;
        $gen = function () use (&$callCount) {
            ++$callCount;
            yield new ThinkingDelta('Hello');
            ++$callCount;
            yield new TextDelta('World');
        };

        $execution = new Execution(function () use ($gen) {
            yield new Result(new StreamResult($gen()));
        }, true);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process($execution);

        $this->assertSame(2, $callCount);
    }

    #[Test]
    public function itInvokesTextResultProcessorCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        $execution = $this->createTextExecution('Hello World');

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            textResultProcessor: function (TextResult $result) use (&$isCalled, &$capturedResult): void {
                $isCalled = true;
                $capturedResult = $result;
            },
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(TextResult::class, $capturedResult);
        $this->assertSame('Hello World', $capturedResult->getContent());
    }

    #[Test]
    public function itInvokesOnStreamResultStartCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        $execution = $this->createStreamExecution([
            new ThinkingDelta('Hello'),
            new TextDelta('World'),
        ]);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            onStreamResultStart: function (StreamResult $result) use (&$isCalled, &$capturedResult): void {
                $isCalled = true;
                $capturedResult = $result;
            },
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(StreamResult::class, $capturedResult);
    }

    #[Test]
    public function itInvokesOnStreamResultFinishCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        $execution = $this->createStreamExecution([
            new ThinkingDelta('Hello'),
            new TextDelta('World'),
        ]);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            onStreamResultFinish: function (StreamResult $result) use (&$isCalled, &$capturedResult): void {
                $isCalled = true;
                $capturedResult = $result;
            },
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(StreamResult::class, $capturedResult);
    }

    #[Test]
    public function itInvokesThinkingDeltaProcessorCallback(): void
    {
        $isCalled = false;
        $capturedDelta = null;

        $execution = $this->createStreamExecution([
            new ThinkingDelta('Hello'),
            new TextDelta('World'),
        ]);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            thinkingDeltaProcessor: function (ThinkingDelta $delta) use (&$isCalled, &$capturedDelta): void {
                $isCalled = true;
                $capturedDelta = $delta;
            },
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(ThinkingDelta::class, $capturedDelta);
        $this->assertSame('Hello', $capturedDelta->getThinking());
    }

    #[Test]
    public function itInvokesTextDeltaProcessorCallback(): void
    {
        $isCalled = false;
        $capturedDelta = null;

        $execution = $this->createStreamExecution([
            new ThinkingDelta('Hello'),
            new TextDelta('World'),
        ]);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            textDeltaProcessor: function (TextDelta $delta) use (&$isCalled, &$capturedDelta): void {
                $isCalled = true;
                $capturedDelta = $delta;
            },
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(TextDelta::class, $capturedDelta);
        $this->assertSame('World', $capturedDelta->getText());
    }

    #[Test]
    public function itInvokesOnProgressCallback(): void
    {
        $capturedProgress = [];
        $progress1 = new Progress('tool_call', 'Searching files...');
        $progress2 = new Progress('tool_call', 'Reading file...');

        $factory = function () use ($progress1, $progress2) {
            yield $progress1;
            yield $progress2;
            yield new Result(new TextResult('Completed'));
        };

        $execution = new Execution($factory, false);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            onProgress: function (Progress $progress) use (&$capturedProgress): void {
                $capturedProgress[] = $progress;
            },
        );

        $this->assertCount(2, $capturedProgress);
        $this->assertSame($progress1, $capturedProgress[0]);
        $this->assertSame($progress2, $capturedProgress[1]);
    }

    #[Test]
    public function itInvokesOnResultCallback(): void
    {
        $capturedResult = null;
        $expectedResultUpdate = new Result(new TextResult('Done'));

        $factory = function () use ($expectedResultUpdate) {
            yield $expectedResultUpdate;
        };

        $execution = new Execution($factory, false);

        $processor = new Processor(new ResultProcessor(new StreamProcessor()));
        $processor->process(
            $execution,
            onResult: function (Result $result) use (&$capturedResult): void {
                $capturedResult = $result;
            },
        );

        $this->assertSame($expectedResultUpdate, $capturedResult);
    }

    private function createTextExecution(string $content): Execution
    {
        $factory = function () use ($content) {
            yield new Result(new TextResult($content));
        };

        return new Execution($factory, false);
    }

    /**
     * @param array<int, TextDelta|ThinkingDelta> $deltas
     */
    private function createStreamExecution(array $deltas): Execution
    {
        $factory = function () use ($deltas) {
            $streamGenerator = function () use ($deltas) {
                foreach ($deltas as $delta) {
                    yield $delta;
                }
            };

            yield new Result(new StreamResult($streamGenerator()));
        };

        return new Execution($factory, true);
    }
}
