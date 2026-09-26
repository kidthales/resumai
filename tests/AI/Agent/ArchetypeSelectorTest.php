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

namespace App\Tests\AI\Agent;

use App\AI\Agent\ArchetypeSelection;
use App\AI\Agent\ArchetypeSelector;
use App\AI\Tool\ArchetypeDirectoryTool;
use App\AI\Tool\Exception\ArchetypeNotFoundException;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
use Symfony\AI\Agent\AgentInterface;
use Symfony\AI\Agent\Execution\Execution;
use Symfony\AI\Agent\Execution\Update\Result;
use Symfony\AI\Platform\Result\TextResult;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
#[Group('ai')]
#[CoversClass(ArchetypeSelector::class)]
#[CoversClass(ArchetypeSelection::class)]
final class ArchetypeSelectorTest extends TestCase
{
    private string $tempDir;

    protected function setUp(): void
    {
        $this->tempDir = sys_get_temp_dir().\DIRECTORY_SEPARATOR.'archetype_selector_test_'.uniqid();
        mkdir($this->tempDir, 0777, true);
    }

    protected function tearDown(): void
    {
        $this->removeDirectory($this->tempDir);
    }

    #[Test]
    public function itSelectsArchetypeFromJobDescriptionWithRawJson(): void
    {
        $archetypeContent = "# Staff Backend Engineer\n\nDistributed systems expert.";
        file_put_contents($this->tempDir.'/staff_backend_engineer.sample.md', $archetypeContent);

        $jobDescription = "Looking for a Staff Backend Engineer with distributed systems expertise.\nTech: Go, PHP, Kafka";
        $agentOutput = json_encode([
            'archetype_id' => 'staff_backend_engineer_sample',
            'archetype_name' => 'Staff Backend Engineer',
            'rationale' => 'Job emphasizes high concurrency distributed architecture.',
        ], \JSON_THROW_ON_ERROR);

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $agentOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($agentOutput);
            });

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $selection = $selector->select($jobDescription);

        $this->assertSame('staff_backend_engineer_sample', $selection->archetypeId);
        $this->assertSame('Staff Backend Engineer', $selection->archetypeName);
        $this->assertSame($archetypeContent, $selection->content);
        $this->assertSame('Job emphasizes high concurrency distributed architecture.', $selection->rationale);

        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString("<job_description>\n{$jobDescription}\n</job_description>", $capturedPrompt);
    }

    #[Test]
    public function itSelectsArchetypeFromJobDescriptionWithFencedJsonAndDerivedTitle(): void
    {
        $archetypeContent = "# Engineering Manager\n\nPeople leadership profile.";
        file_put_contents($this->tempDir.'/engineering_manager.sample.md', $archetypeContent);

        $agentOutput = "```json\n".json_encode([
            'archetype_id' => 'engineering_manager_sample',
            'rationale' => 'Role requires people leadership and hiring strategy.',
        ], \JSON_THROW_ON_ERROR)."\n```";

        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution($agentOutput));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $selection = $selector->select('We need an Engineering Manager for our Platform tribe.');

        $this->assertSame('engineering_manager_sample', $selection->archetypeId);
        $this->assertSame('Engineering Manager Sample', $selection->archetypeName);
        $this->assertSame($archetypeContent, $selection->content);
        $this->assertSame('Role requires people leadership and hiring strategy.', $selection->rationale);
    }

    #[Test]
    public function itSelectsCustomNonSampleArchetypeFromJobDescription(): void
    {
        $archetypeContent = "# Security Architect\n\nCloud and enterprise security.";
        file_put_contents($this->tempDir.'/security_architect.md', $archetypeContent);

        $agentOutput = json_encode([
            'archetype_id' => 'security_architect',
            'archetype_name' => 'Security Architect',
            'rationale' => 'Cybersecurity governance and zero-trust focus.',
        ], \JSON_THROW_ON_ERROR);

        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution($agentOutput));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $selection = $selector->select('Looking for a Security Architect to oversee SOC2 and IAM.');

        $this->assertSame('security_architect', $selection->archetypeId);
        $this->assertSame('Security Architect', $selection->archetypeName);
        $this->assertSame($archetypeContent, $selection->content);
        $this->assertSame('Cybersecurity governance and zero-trust focus.', $selection->rationale);
    }

    #[Test]
    public function itThrowsExceptionWhenJobDescriptionIsEmpty(): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->never())->method('call');

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessageMatches('/Job description cannot be empty\./');

        $selector->select('   ');
    }

    #[Test]
    public function itThrowsExceptionWhenAgentOutputIsNotValidJson(): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution('I think the best archetype is staff_backend_engineer'));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessageMatches('/Failed to parse archetype selection JSON response/');

        $selector->select('Looking for lead developer');
    }

    #[Test]
    public function itThrowsExceptionWhenArchetypeIdIsMissingInOutput(): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution(json_encode(['rationale' => 'No ID provided'], \JSON_THROW_ON_ERROR)));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessageMatches('/Missing or invalid "archetype_id" in agent response/');

        $selector->select('Looking for lead developer');
    }

    #[Test]
    public function itPropagatesNotFoundExceptionWhenSelectedArchetypeDoesNotExist(): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution(json_encode(['archetype_id' => 'non_existent_archetype'], \JSON_THROW_ON_ERROR)));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $this->expectException(ArchetypeNotFoundException::class);
        $this->expectExceptionMessageMatches('/Archetype with identifier "non_existent_archetype" was not found\./');

        $selector->select('Looking for lead developer');
    }

    private function createArchetypeSelector(
        AgentInterface $agent,
        ArchetypeDirectoryTool $tool,
    ): ArchetypeSelector {
        return new ArchetypeSelector($agent, $tool);
    }

    private function createExecution(string $output): Execution
    {
        return new Execution(static function () use ($output): \Generator {
            yield new Result(new TextResult($output));
        });
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
