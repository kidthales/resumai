<?php

declare(strict_types=1);

namespace App\AI\Agent\Toolbox;

use App\AI\Agent\Toolbox\Exception\ArchetypeNotFoundException;
use App\Filesystem\PathChecker;
use Symfony\AI\Agent\Toolbox\Attribute\AsTool;
use Symfony\Component\DependencyInjection\Attribute\Autowire;
use Symfony\Component\Filesystem\Filesystem;
use Symfony\Component\Filesystem\Path;
use Symfony\Component\Finder\Finder;

use function Symfony\Component\String\u;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsTool(
    name: 'list_archetypes',
    description: 'Lists all available candidate archetype filenames in the archetypes directory',
    method: 'listArchetypes',
)]
#[AsTool(
    name: 'read_archetype',
    description: 'Reads the full markdown content of a specific archetype file by its filename',
    method: 'readArchetype',
)]
final readonly class ArchetypeDirectoryTool
{
    public function __construct(
        #[Autowire('%archetypes_path%')] private string $archetypesPath,
        private Filesystem $filesystem = new Filesystem(),
    ) {
    }

    /**
     * Lists all available candidate archetype filenames.
     *
     * @return list<string>
     */
    public function listArchetypes(): array
    {
        if (!$this->filesystem->exists($this->archetypesPath)) {
            return [];
        }

        $finder = new Finder()
            ->files()
            ->in($this->archetypesPath)
            ->depth('== 0')
            ->name('*.md')
            ->ignoreDotFiles(true)
            ->sortByName();

        $samples = [];
        $nonSamples = [];
        foreach ($finder as $file) {
            $filename = u($file->getFilename())->trim();
            if ($filename->endsWith('.sample.md')) {
                $samples[] = $filename->toString();
            } else {
                $nonSamples[] = $filename->toString();
            }
        }

        return array_values(0 < count($nonSamples) ? $nonSamples : $samples);
    }

    /**
     * Reads the full Markdown content of a specific archetype file.
     *
     * @param string $filename The filename of the archetype to read (e.g. "staff_backend_engineer.sample.md")
     *
     * @throws \InvalidArgumentException  When the archetype filename is empty or contains path traversal sequences
     * @throws ArchetypeNotFoundException When the archetype file cannot be found
     */
    public function readArchetype(string $filename): string
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

        // Path::join gracefully handles slashes
        $targetPath = Path::join($this->archetypesPath, $candidate);

        $pathChecker = new PathChecker($targetPath);
        if ($pathChecker->hasBasepath($this->archetypesPath, useRealpath: true)) {
            return $this->filesystem->readFile($pathChecker->realpath());
        }

        throw ArchetypeNotFoundException::forFilename($filename);
    }
}
