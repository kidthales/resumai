<?php

declare(strict_types=1);

namespace App\Filesystem;

use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\Filesystem\Path;
use Symfony\Component\Finder\Finder;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final class Filesystem extends \Symfony\Component\Filesystem\Filesystem
{
    public function __construct(
        #[Autowire('%kernel.project_dir%')] private readonly string $projectPath,
        #[Autowire('%archetypes_path%')] private readonly string $archetypesPath,
    ) {
    }

    public function realProjectPath(string $path): string
    {
        $pathChecker = new PathChecker($path);

        if ($pathChecker->isEmpty()) {
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

        if ($pathChecker->isEmpty()) {
            throw new \RuntimeException('Path cannot be empty.');
        }

        $realProjectPath = realpath($this->projectPath) ?: $this->projectPath;

        // 1. Get the logical absolute path first
        $logicalPath = Path::makeAbsolute($pathChecker->path(), $realProjectPath);

        // 2. Find the deepest portion of the path that actually exists on disk
        $existingPart = $logicalPath;
        $nonExistingPart = [];
        while (!$this->exists($existingPart) && $existingPart !== dirname($existingPart)) {
            array_unshift($nonExistingPart, basename($existingPart));
            $existingPart = dirname($existingPart);
        }

        // 3. Safely resolve any symlinks in the existing portion
        if ($this->exists($existingPart)) {
            $existingPart = realpath($existingPart) ?: $existingPart;
        }

        // 4. Reconstruct the true target path
        $finalPath = empty($nonExistingPart)
            ? $existingPart
            : Path::join($existingPart, ...$nonExistingPart);

        // 5. Verify the physical path remains within the project boundary
        if (!Path::isBasePath($realProjectPath, $finalPath)) {
            throw new \RuntimeException(\sprintf('Path "%s" resolves outside the project path.', $path));
        }

        return $finalPath;
    }

    public function findArchetypeFiles(): array|Finder
    {
        if (!$this->exists($this->archetypesPath)) {
            return [];
        }

        return new Finder()
            ->files()
            ->in($this->archetypesPath)
            ->depth('== 0')
            ->name('*.md')
            ->ignoreDotFiles(true)
            ->sortByName();
    }

    public function readArchetypeFile(string $filename): string
    {
        $trimmedFilename = u($filename)->trim();

        if ($trimmedFilename->isEmpty()) {
            throw new \RuntimeException('Archetype filename cannot be empty.');
        }

        if ($trimmedFilename->containsAny(['..', '/', '\\', "\0"])) {
            throw new \RuntimeException(\sprintf('Invalid archetype filename: "%s".', $filename));
        }

        $candidate = $trimmedFilename->endsWith('.md')
            ? $trimmedFilename->toString()
            : $trimmedFilename->append('.md')->toString();

        $realCandidatePath = $this->realProjectPath(Path::join($this->archetypesPath, $candidate));

        return $this->readFile($realCandidatePath);
    }
}
