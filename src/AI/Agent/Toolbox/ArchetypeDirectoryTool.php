<?php

declare(strict_types=1);

namespace App\AI\Agent\Toolbox;

use App\Filesystem\FilesystemV2;
use Symfony\AI\Agent\Toolbox\Attribute\AsTool;

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
    public function __construct(private FilesystemV2 $filesystem)
    {
    }

    public function listArchetypes(): array
    {
        $finder = $this->filesystem->findArchetypeFiles();

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

    public function readArchetype(string $filename): string
    {
        return $this->filesystem->readArchetypeFile($filename);
    }
}
