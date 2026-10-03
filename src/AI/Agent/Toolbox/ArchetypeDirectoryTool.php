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

namespace App\AI\Agent\Toolbox;

use App\Filesystem\Filesystem;
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
    public function __construct(private Filesystem $filesystem)
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
