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

namespace App\AI\Platform\Result\Stream\Exception;

/**
 * Thrown when an unsupported or unexpected DeltaInterface implementation is encountered during stream processing.
 *
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final class UnexpectedDeltaTypeException extends \RuntimeException
{
    public static function forDelta(mixed $delta): self
    {
        $type = \is_object($delta) ? $delta::class : \gettype($delta);

        return new self(\sprintf('Unexpected stream result content delta type: "%s".', $type));
    }
}
