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

namespace App\Tests\AI\Tool;

use App\AI\Agent\Exception\ArchetypeNotFoundException;
use App\AI\Tool\ArchetypeDirectoryTool;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[Group('ai')]
#[CoversClass(ArchetypeDirectoryTool::class)]
#[CoversClass(ArchetypeNotFoundException::class)]
final class ArchetypeDirectoryToolTest extends TestCase
{
    private string $tempDir;

    protected function setUp(): void
    {
        $this->tempDir = sys_get_temp_dir().\DIRECTORY_SEPARATOR.'archetype_test_'.uniqid();
        mkdir($this->tempDir, 0777, true);
    }

    protected function tearDown(): void
    {
        $this->removeDirectory($this->tempDir);
    }

    #[Test]
    public function itListsAllArchetypesInDirectorySortedById(): void
    {
        file_put_contents($this->tempDir.'/staff_backend_engineer.sample.md', "# Staff Backend Engineer\n\nOverview content.");
        file_put_contents($this->tempDir.'/devops_sre.sample.md', "# DevOps & SRE\n\nDevOps content.");
        file_put_contents($this->tempDir.'/engineering_manager.md', "# Engineering Manager\n\nEM content.");
        file_put_contents($this->tempDir.'/ignored.txt', 'Not markdown');
        file_put_contents($this->tempDir.'/.hidden.md', 'Hidden file');

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $archetypes = $tool->listArchetypes();

        $this->assertCount(3, $archetypes);
        $this->assertSame([
            [
                'id' => 'devops_sre_sample',
                'filename' => 'devops_sre.sample.md',
                'title' => 'DevOps & SRE',
            ],
            [
                'id' => 'engineering_manager',
                'filename' => 'engineering_manager.md',
                'title' => 'Engineering Manager',
            ],
            [
                'id' => 'staff_backend_engineer_sample',
                'filename' => 'staff_backend_engineer.sample.md',
                'title' => 'Staff Backend Engineer',
            ],
        ], $archetypes);
    }

    #[Test]
    public function itExtractsTitleFromFirstHeadingOrFormatsFromId(): void
    {
        file_put_contents($this->tempDir.'/custom_role.sample.md', "No heading here.\nJust paragraphs.");

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $archetypes = $tool->listArchetypes();

        $this->assertCount(1, $archetypes);
        $this->assertSame('custom_role_sample', $archetypes[0]['id']);
        $this->assertSame('Custom Role Sample', $archetypes[0]['title']);
    }

    #[Test]
    public function itListsAndResolvesBothSampleAndCustomArchetypesWhenCoexisting(): void
    {
        $sampleContent = "# Platform Engineer\n\nSample platform engineer profile.";
        $customContent = "# Platform Engineer\n\nCustom company-tailored platform engineer profile.";

        file_put_contents($this->tempDir.'/platform_engineer.sample.md', $sampleContent);
        file_put_contents($this->tempDir.'/platform_engineer.md', $customContent);

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $archetypes = $tool->listArchetypes();

        $this->assertCount(2, $archetypes);
        $this->assertSame([
            [
                'id' => 'platform_engineer',
                'filename' => 'platform_engineer.md',
                'title' => 'Platform Engineer',
            ],
            [
                'id' => 'platform_engineer_sample',
                'filename' => 'platform_engineer.sample.md',
                'title' => 'Platform Engineer',
            ],
        ], $archetypes);

        $this->assertSame($customContent, $tool->readArchetype('platform_engineer'));
        $this->assertSame($sampleContent, $tool->readArchetype('platform_engineer_sample'));
        $this->assertSame($customContent, $tool->readArchetype('platform_engineer.md'));
        $this->assertSame($sampleContent, $tool->readArchetype('platform_engineer.sample.md'));
    }

    #[Test]
    public function itReturnsEmptyListForNonExistentDirectory(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir.'/non_existent_folder');
        $this->assertSame([], $tool->listArchetypes());
    }

    #[Test]
    public function itReadsArchetypeContentByIdOrFilename(): void
    {
        $content = "# Staff Backend Engineer\n\nDeep systems expertise.";
        file_put_contents($this->tempDir.'/staff_backend_engineer.sample.md', $content);

        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->assertSame($content, $tool->readArchetype('staff_backend_engineer_sample'));
        $this->assertSame($content, $tool->readArchetype('staff_backend_engineer'));
        $this->assertSame($content, $tool->readArchetype('staff_backend_engineer.sample.md'));
    }

    #[Test]
    public function itThrowsExceptionWhenArchetypeIdIsEmpty(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Archetype identifier cannot be empty.');

        $tool->readArchetype('   ');
    }

    #[Test]
    #[DataProvider('providePathTraversalPayloads')]
    public function itThrowsExceptionOnPathTraversalAttempt(string $maliciousId): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Invalid archetype identifier');

        $tool->readArchetype($maliciousId);
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function providePathTraversalPayloads(): iterable
    {
        yield 'parent directory dots' => ['../secret.txt'];
        yield 'double dot nested' => ['subdir/../../secret'];
        yield 'absolute path unix' => ['/etc/passwd'];
        yield 'windows path backslash' => ['C:\\Windows\\System32'];
        yield 'slash separation' => ['sub/file'];
        yield 'null byte injection' => ["file\0.md"];
    }

    #[Test]
    public function itThrowsNotFoundExceptionWhenArchetypeDoesNotExist(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(ArchetypeNotFoundException::class);
        $this->expectExceptionMessage('Archetype with identifier "unknown_role" was not found.');

        $tool->readArchetype('unknown_role');
    }

    private function removeDirectory(string $dir): void
    {
        if (!is_dir($dir)) {
            return;
        }

        $files = scandir($dir);
        if (false !== $files) {
            foreach ($files as $file) {
                if ('.' === $file || '..' === $file) {
                    continue;
                }
                $path = $dir.\DIRECTORY_SEPARATOR.$file;
                if (is_dir($path)) {
                    $this->removeDirectory($path);
                } else {
                    unlink($path);
                }
            }
        }
        rmdir($dir);
    }
}
