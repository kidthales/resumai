<?php

declare(strict_types=1);

namespace App\Filesystem;

use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\Filesystem\Path;

final class FilesystemV2 extends \Symfony\Component\Filesystem\Filesystem
{
    public function __construct(#[Autowire('%kernel.project_dir%')] private readonly string $projectPath)
    {
    }

    public function realProjectPath(string $path): string
    {
        $pathChecker = new PathChecker($path);

        if ('' === $pathChecker->path()) {
            throw new \RuntimeException('Path cannot be empty.');
        }

        if (!($realProjectPath = $pathChecker->realpath())) {
            throw new \RuntimeException(\sprintf('Path "%s" was not found.', $path));
        }

        if (!$pathChecker->hasBasepath($this->projectPath, useRealpath: true)) {
            throw new \RuntimeException(\sprintf('Path "%s" is not within the project path.', $path));
        }

        return $realProjectPath;
    }

    public function resolveProjectPath(string $path): string
    {
        $pathChecker = new PathChecker($path);

        if ('' === $pathChecker->path()) {
            throw new \RuntimeException('Path cannot be empty.');
        }

        if ($pathChecker->isAbsolute()) {
            if (!$pathChecker->hasBasepath($this->projectPath)) {
                throw new \RuntimeException(\sprintf('Path "%s" is not within the project path.', $path));
            }

            return $pathChecker->canonicalize();
        }

        $joinedPathChecker = new PathChecker(Path::join($this->projectPath, $pathChecker->path()));

        // TODO: What about symlinks?
        if (!$joinedPathChecker->hasBasepath($this->projectPath)) {
            throw new \RuntimeException(\sprintf('Path "%s" is not within the project path.', $path));
        }

        return $joinedPathChecker->canonicalize();
    }
}
