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

namespace App\Tests\AI\Platform\Result;

use App\AI\Platform\Result\Processor;
use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
use Symfony\AI\Platform\Metadata\Metadata;
use Symfony\AI\Platform\Result\RawResultInterface;
use Symfony\AI\Platform\Result\ResultInterface;
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
    public function itProcessesResultWithNoCallbacks(): void
    {
        $callCount = 0;
        $gen = function () use (&$callCount) {
            ++$callCount;
            yield new ThinkingDelta('Hello');
            ++$callCount;
            yield new TextDelta('World');
        };

        new Processor(new StreamProcessor())->process(new StreamResult($gen()));

        $this->assertSame(2, $callCount);
    }

    #[Test]
    public function itInvokesTextResultProcessorCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        new Processor(new StreamProcessor())
            ->process(new TextResult('Hello World'), function ($result) use (&$isCalled, &$capturedResult) {
                $isCalled = true;
                $capturedResult = $result;
            });

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(TextResult::class, $capturedResult);
        $this->assertSame('Hello World', $capturedResult->getContent());
    }

    #[Test]
    public function itInvokesOnStreamResultStartCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        new Processor(new StreamProcessor())
            ->process(
                $this->createStreamResult(),
                onStreamResultStart: function ($result) use (&$isCalled, &$capturedResult) {
                    $isCalled = true;
                    $capturedResult = $result;
                }
            );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(StreamResult::class, $capturedResult);
    }

    #[Test]
    public function itInvokesOnStreamResultFinishCallback(): void
    {
        $isCalled = false;
        $capturedResult = null;

        new Processor(new StreamProcessor())
            ->process(
                $this->createStreamResult(),
                onStreamResultFinish: function ($result) use (&$isCalled, &$capturedResult) {
                    $isCalled = true;
                    $capturedResult = $result;
                }
            );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(StreamResult::class, $capturedResult);
    }

    #[Test]
    public function itInvokesThinkingDeltaProcessorCallback(): void
    {
        $isCalled = false;
        $capturedDelta = null;

        new Processor(new StreamProcessor())
            ->process(
                $this->createStreamResult(),
                thinkingDeltaProcessor: function ($delta) use (&$isCalled, &$capturedDelta) {
                    $isCalled = true;
                    $capturedDelta = $delta;
                }
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

        new Processor(new StreamProcessor())
            ->process(
                $this->createStreamResult(),
                textDeltaProcessor: function ($delta) use (&$isCalled, &$capturedDelta) {
                    $isCalled = true;
                    $capturedDelta = $delta;
                }
            );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(TextDelta::class, $capturedDelta);
        $this->assertSame('World', $capturedDelta->getText());
    }

    #[Test]
    public function itThrowsExceptionWithUnsupportedResultInterfaceImplementation(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessageMatches('/Unexpected result type/');

        new Processor(new StreamProcessor())->process(new class implements ResultInterface {
            public function getMetadata(): Metadata
            {
                return new Metadata();
            }

            public function getContent(): string|iterable|object|null
            {
                return 'Hello World';
            }

            public function getRawResult(): ?RawResultInterface
            {
                return null;
            }

            public function setRawResult(RawResultInterface $rawResult): void
            {
            }
        });
    }

    private function createStreamResult(): StreamResult
    {
        $gen = function () {
            yield new ThinkingDelta('Hello');
            yield new TextDelta('World');
        };

        return new StreamResult($gen());
    }
}
