<?php

declare(strict_types=1);

namespace App\Tests\AI\Agent\Toolbox;

use App\AI\Agent\Toolbox\ArchetypeDirectoryTool;
use App\AI\Agent\Toolbox\Exception\ArchetypeNotFoundException;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
use Symfony\Component\Filesystem\Filesystem;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[Group('ai')]
#[CoversClass(ArchetypeDirectoryTool::class)]
#[CoversClass(ArchetypeNotFoundException::class)]
final class ArchetypeDirectoryToolTest extends TestCase
{
    private string $tempDir;
    private Filesystem $filesystem;

    protected function setUp(): void
    {
        $this->filesystem = new Filesystem();

        // Cryptographically random suffix prevents any parallel collisions
        $this->tempDir = sys_get_temp_dir().\DIRECTORY_SEPARATOR.'archetype_test_'.bin2hex(random_bytes(8));
        $this->filesystem->mkdir($this->tempDir);
    }

    protected function tearDown(): void
    {
        // Filesystem::remove recursively deletes files and folders robustly
        $this->filesystem->remove($this->tempDir);
    }

    #[Test]
    public function itListsAllArchetypeFilenamesInDirectorySortedAlphabetically(): void
    {
        $this->filesystem->appendToFile($this->tempDir.'/staff_backend_engineer.sample.md', "# Staff Backend Engineer\n\nOverview content.");
        $this->filesystem->appendToFile($this->tempDir.'/devops_sre.sample.md', "# DevOps & SRE\n\nDevOps content.");
        $this->filesystem->appendToFile($this->tempDir.'/engineering_manager.md', "# Engineering Manager\n\nEM content.");
        $this->filesystem->appendToFile($this->tempDir.'/ignored.txt', 'Not markdown');
        $this->filesystem->appendToFile($this->tempDir.'/.hidden.md', 'Hidden file');

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $archetypes = $tool->listArchetypes();

        $this->assertCount(3, $archetypes);
        $this->assertSame([
            'devops_sre.sample.md',
            'engineering_manager.md',
            'staff_backend_engineer.sample.md',
        ], $archetypes);
    }

    #[Test]
    public function itListsAndResolvesBothSampleAndCustomArchetypesWhenCoexisting(): void
    {
        $sampleContent = "# Platform Engineer\n\nSample platform engineer profile.";
        $customContent = "# Platform Engineer\n\nCustom company-tailored platform engineer profile.";

        $this->filesystem->appendToFile($this->tempDir.'/platform_engineer.sample.md', $sampleContent);
        $this->filesystem->appendToFile($this->tempDir.'/platform_engineer.md', $customContent);

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $archetypes = $tool->listArchetypes();

        $this->assertCount(2, $archetypes);
        $this->assertSame([
            'platform_engineer.md',
            'platform_engineer.sample.md',
        ], $archetypes);

        $this->assertSame($customContent, $tool->readArchetype('platform_engineer'));
        $this->assertSame($customContent, $tool->readArchetype('platform_engineer.md'));
        $this->assertSame($sampleContent, $tool->readArchetype('platform_engineer.sample'));
        $this->assertSame($sampleContent, $tool->readArchetype('platform_engineer.sample.md'));
    }

    #[Test]
    public function itReturnsEmptyListForNonExistentDirectory(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir.'/non_existent_folder');
        $this->assertSame([], $tool->listArchetypes());
    }

    #[Test]
    public function itReadsArchetypeContentByExactFilenameOrExtensionlessFilename(): void
    {
        $content = "# Staff Backend Engineer\n\nDeep systems expertise.";
        $this->filesystem->appendToFile($this->tempDir.'/staff_backend_engineer.sample.md', $content);

        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->assertSame($content, $tool->readArchetype('staff_backend_engineer.sample.md'));
        $this->assertSame($content, $tool->readArchetype('staff_backend_engineer.sample'));
    }

    #[Test]
    public function itThrowsExceptionWhenArchetypeFilenameIsEmpty(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessageMatches('/Archetype filename cannot be empty\./');

        $tool->readArchetype('   ');
    }

    #[Test]
    #[DataProvider('providePathTraversalPayloads')]
    public function itThrowsExceptionOnPathTraversalAttempt(string $maliciousFilename): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessageMatches('/Invalid archetype filename/');

        $tool->readArchetype($maliciousFilename);
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
        $this->expectExceptionMessageMatches('/Archetype with filename "unknown_role\.md" was not found\./');

        $tool->readArchetype('unknown_role.md');
    }

    #[Test]
    public function itThrowsNotFoundExceptionWhenExtensionlessArchetypeDoesNotExist(): void
    {
        $tool = new ArchetypeDirectoryTool($this->tempDir);

        $this->expectException(ArchetypeNotFoundException::class);
        $this->expectExceptionMessageMatches('/Archetype with filename "unknown_role" was not found\./');

        $tool->readArchetype('unknown_role');
    }
}
