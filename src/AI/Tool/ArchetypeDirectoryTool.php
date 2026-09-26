<?php

/*
 * This file is part of the ResumAI package.
 *
 * (c) Tristan Bonsor <kidthales@agogpixel.com>
 *
 * For the full copyright and license information, please view the LICENSE
 * file that was distributed with this source code.
 */

declare(strict_types=1);

namespace App\AI\Tool;

use App\AI\Tool\Exception\ArchetypeNotFoundException;
use Symfony\AI\Agent\Toolbox\Attribute\AsTool;
use Symfony\Component\DependencyInjection\Attribute\Autowire;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[AsTool(
    name: 'list_archetypes',
    description: 'Lists all available candidate archetype filenames and identifiers in the archetypes directory',
    method: 'listArchetypes',
)]
#[AsTool(
    name: 'read_archetype',
    description: 'Reads the full markdown content of a specific archetype file by its filename or identifier',
    method: 'readArchetype',
)]
final readonly class ArchetypeDirectoryTool
{
    public function __construct(
        #[Autowire('%archetypes_path%')]
        private string $archetypesPath,
    ) {
    }

    /**
     * Lists all available candidate archetype identifiers, filenames, and titles.
     *
     * @return list<array{id: string, filename: string, title: string}>
     */
    public function listArchetypes(): array
    {
        if (!is_dir($this->archetypesPath)) {
            return [];
        }

        $files = scandir($this->archetypesPath);
        if (false === $files) {
            return [];
        }

        $archetypes = [];
        foreach ($files as $file) {
            if ('.' === $file || '..' === $file || str_starts_with($file, '.')) {
                continue;
            }

            if (!str_ends_with($file, '.md')) {
                continue;
            }

            $filePath = $this->archetypesPath.\DIRECTORY_SEPARATOR.$file;
            if (!is_file($filePath)) {
                continue;
            }

            if (str_ends_with($file, '.sample.md')) {
                $id = (string) preg_replace('/\.sample\.md$/', '_sample', $file);
            } else {
                $id = (string) preg_replace('/\.md$/', '', $file);
            }
            $title = $this->extractTitle($filePath) ?? $this->formatTitleFromId($id);

            $archetypes[] = [
                'id' => $id,
                'filename' => $file,
                'title' => $title,
            ];
        }

        usort($archetypes, static fn (array $a, array $b): int => strcmp($a['id'], $b['id']));

        return array_values($archetypes);
    }

    /**
     * Reads the full markdown content of a specific archetype file.
     *
     * @param string $archetypeId The filename or identifier of the archetype to read
     *
     * @throws \InvalidArgumentException  When the archetype ID is invalid or contains path traversal sequences
     * @throws ArchetypeNotFoundException When the archetype cannot be found
     */
    public function readArchetype(string $archetypeId): string
    {
        $trimmedId = trim($archetypeId);
        if ('' === $trimmedId) {
            throw new \InvalidArgumentException('Archetype identifier cannot be empty.');
        }

        if (str_contains($trimmedId, '..') || str_contains($trimmedId, '/') || str_contains($trimmedId, '\\') || str_contains($trimmedId, "\0")) {
            throw new \InvalidArgumentException(\sprintf('Invalid archetype identifier: "%s".', $archetypeId));
        }

        $candidates = [$trimmedId];

        if (str_ends_with($trimmedId, '_sample')) {
            $base = substr($trimmedId, 0, -7);
            $candidates[] = $base.'.sample.md';
        }

        $candidates[] = $trimmedId.'.md';
        $candidates[] = $trimmedId.'.sample.md';

        foreach ($candidates as $candidate) {
            $targetPath = $this->archetypesPath.\DIRECTORY_SEPARATOR.$candidate;
            if (is_file($targetPath) && is_readable($targetPath)) {
                $realTarget = realpath($targetPath);
                $realBase = realpath($this->archetypesPath);

                if (false !== $realTarget && false !== $realBase && str_starts_with($realTarget, $realBase)) {
                    $content = file_get_contents($realTarget);
                    if (false !== $content) {
                        return $content;
                    }
                }
            }
        }

        throw ArchetypeNotFoundException::forId($archetypeId);
    }

    private function extractTitle(string $filePath): ?string
    {
        $handle = @fopen($filePath, 'r');
        if (false === $handle) {
            return null;
        }

        try {
            while (false !== ($line = fgets($handle))) {
                $line = trim($line);
                if (str_starts_with($line, '# ')) {
                    return trim(substr($line, 2));
                }
            }
        } finally {
            fclose($handle);
        }

        return null;
    }

    private function formatTitleFromId(string $id): string
    {
        return ucwords(str_replace(['_', '-'], ' ', $id));
    }
}
