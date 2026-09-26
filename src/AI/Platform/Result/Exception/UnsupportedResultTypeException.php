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

namespace App\AI\Platform\Result\Exception;

/**
 * Thrown when an unsupported ResultInterface implementation is provided to the result processor.
 *
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final class UnsupportedResultTypeException extends \InvalidArgumentException
{
    public static function forResult(mixed $result): self
    {
        $type = \is_object($result) ? $result::class : \gettype($result);

        return new self(\sprintf('Unexpected result type: "%s".', $type));
    }
}
