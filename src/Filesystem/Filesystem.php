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
            throw new \InvalidArgumentException('Archetype filename cannot be empty.');
        }

        if ($trimmedFilename->containsAny(['..', '/', '\\', "\0"])) {
            throw new \InvalidArgumentException(\sprintf('Invalid archetype filename: "%s".', $filename));
        }

        $candidate = $trimmedFilename->endsWith('.md')
            ? $trimmedFilename->toString()
            : $trimmedFilename->append('.md')->toString();

        $realCandidatePath = $this->realProjectPath(Path::join($this->archetypesPath, $candidate));

        return $this->readFile($realCandidatePath);
    }
}
