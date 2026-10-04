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

namespace App\Filesystem;

use Symfony\Component\Filesystem\Path;
use Symfony\Component\String\UnicodeString;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class PathChecker
{
    private UnicodeString $path;

    public function __construct(string $path)
    {
        $this->path = u($path)->trim();
    }

    public function path(): string
    {
        return $this->path->toString();
    }

    public function realpath(): false|string
    {
        return realpath($this->path->toString());
    }

    public function hasBasepath(string $basepath, $useRealpath = false): bool
    {
        $basepathChecker = new self($basepath);

        if ($useRealpath) {
            $realBasepath = $basepathChecker->realpath();
            $realpath = $this->realpath();

            return $realBasepath && $realpath && Path::isBasePath($realBasepath, $realpath);
        }

        $basepath = $basepathChecker->path();
        $path = $this->path();

        return '' !== $basepath && '' !== $path && Path::isBasePath($basepath, $path);
    }

    public function isEmpty(): bool
    {
        return $this->path->isEmpty();
    }
}
