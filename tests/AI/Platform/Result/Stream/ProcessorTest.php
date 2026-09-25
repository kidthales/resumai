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

namespace App\Tests\Unit\Domain\Shared\AI;

use App\AI\Platform\Result\Stream\Processor;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
use Symfony\AI\Platform\Result\Stream\Delta\DeltaInterface;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;

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

        new Processor()->process(new StreamResult($gen()));

        $this->assertSame(2, $callCount);
    }

    #[Test]
    public function itInvokesOnStartCallback(): void
    {
        $gen = function () {
            yield new ThinkingDelta('Hello');
            yield new TextDelta('World');
        };

        $isCalled = false;
        $capturedResult = null;

        new Processor()->process(
            new StreamResult($gen()),
            onStreamResultStart: function ($result) use (&$isCalled, &$capturedResult) {
                $isCalled = true;
                $capturedResult = $result;
            }
        );

        $this->assertTrue($isCalled);
        $this->assertInstanceOf(StreamResult::class, $capturedResult);
    }

    #[Test]
    public function itInvokesOnFinishCallback(): void
    {
        $gen = function () {
            yield new ThinkingDelta('Hello');
            yield new TextDelta('World');
        };

        $isCalled = false;
        $capturedResult = null;

        new Processor()->process(
            new StreamResult($gen()),
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
        $gen = function () {
            yield new ThinkingDelta('Hello');
            yield new TextDelta('World');
        };

        $isCalled = false;
        $capturedDelta = null;

        new Processor()->process(
            new StreamResult($gen()),
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
        $gen = function () {
            yield new ThinkingDelta('Hello');
            yield new TextDelta('World');
        };

        $isCalled = false;
        $capturedDelta = null;

        new Processor()->process(
            new StreamResult($gen()),
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
    public function itThrowsExceptionWithUnsupportedDeltaInterfaceImplementation(): void
    {
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessageMatches('/Unexpected stream result content delta type/');

        $gen = function () {
            yield new class implements DeltaInterface {
                public function getType(): string
                {
                    return 'unknown';
                }

                public function toArray(): array
                {
                    return [];
                }
            };
        };

        new Processor()->process(new StreamResult($gen()));
    }

    #[Test]
    public function itHandlesEmptyStreamGracefully(): void
    {
        $gen = function () {
            yield from [];
        };
        $isCalled = false;

        new Processor()->process(
            new StreamResult($gen()),
            onStreamResultFinish: function () use (&$isCalled) {
                $isCalled = true;
            }
        );

        $this->assertTrue($isCalled);
    }
}
