<?php

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
