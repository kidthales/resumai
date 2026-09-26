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
use App\AI\Agent\Execution\Processor as ExecutionProcessor;
use App\AI\Platform\Result\Processor as ResultProcessor;
use App\AI\Platform\Result\Stream\Processor as StreamProcessor;
use App\AI\Tool\ArchetypeDirectoryTool;
use App\AI\Tool\Exception\ArchetypeNotFoundException;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\Attributes\Test;
use PHPUnit\Framework\TestCase;
use Symfony\AI\Agent\AgentInterface;
use Symfony\AI\Agent\Execution\Execution;
use Symfony\AI\Agent\Execution\Update\Result;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\AI\Platform\Result\StreamResult;
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
    public function itIsolatesThinkingDeltasAndInvokesCallbacksWhenStreamed(): void
    {
        $archetypeContent = "# Full-Stack Engineer\n\nEnd to end feature delivery.";
        file_put_contents($this->tempDir.'/fullstack_engineer.sample.md', $archetypeContent);

        $jsonPayload = json_encode([
            'archetype_id' => 'fullstack_engineer_sample',
            'archetype_name' => 'Full-Stack Engineer',
            'rationale' => 'Requires both React and backend APIs.',
        ], \JSON_THROW_ON_ERROR);

        $deltas = [
            new ThinkingDelta('Evaluating candidate archetypes: backend vs fullstack...'),
            new ThinkingDelta('Selected fullstack_engineer_sample based on frontend requirements.'),
            new TextDelta(substr($jsonPayload, 0, 20)),
            new TextDelta(substr($jsonPayload, 20)),
        ];

        $capturedThinking = [];
        $capturedText = [];

        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createStreamExecution($deltas));

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $selection = $selector->select(
            jobDescription: 'Frontend React + Node developer needed.',
            thinkingDeltaProcessor: function (ThinkingDelta $delta) use (&$capturedThinking): void {
                $capturedThinking[] = $delta->getThinking();
            },
            textDeltaProcessor: function (TextDelta $delta) use (&$capturedText): void {
                $capturedText[] = $delta->getText();
            },
        );

        $this->assertSame('fullstack_engineer_sample', $selection->archetypeId);
        $this->assertSame($archetypeContent, $selection->content);
        $this->assertSame('Requires both React and backend APIs.', $selection->rationale);

        $this->assertCount(2, $capturedThinking);
        $this->assertSame(['Evaluating candidate archetypes: backend vs fullstack...', 'Selected fullstack_engineer_sample based on frontend requirements.'], $capturedThinking);
        $this->assertSame([substr($jsonPayload, 0, 20), substr($jsonPayload, 20)], $capturedText);
    }

    #[Test]
    public function itThrowsExceptionWhenJobDescriptionIsEmpty(): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->never())->method('call');

        $tool = new ArchetypeDirectoryTool($this->tempDir);
        $selector = $this->createArchetypeSelector($agent, $tool);

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Job description cannot be empty.');

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
        $this->expectExceptionMessage('Failed to parse archetype selection JSON response');

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
        $this->expectExceptionMessage('Missing or invalid "archetype_id" in agent response');

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
        $this->expectExceptionMessage('Archetype with identifier "non_existent_archetype" was not found.');

        $selector->select('Looking for lead developer');
    }

    private function createArchetypeSelector(
        AgentInterface $agent,
        ArchetypeDirectoryTool $tool,
        ?ExecutionProcessor $processor = null,
    ): ArchetypeSelector {
        return new ArchetypeSelector(
            $agent,
            $processor ?? new ExecutionProcessor(new ResultProcessor(new StreamProcessor())),
            $tool,
        );
    }

    private function createExecution(string $output): Execution
    {
        return new Execution(static function () use ($output): \Generator {
            yield new Result(new TextResult($output));
        });
    }

    /**
     * @param list<TextDelta|ThinkingDelta> $deltas
     */
    private function createStreamExecution(array $deltas): Execution
    {
        return new Execution(static function () use ($deltas): \Generator {
            $gen = static function () use ($deltas): \Generator {
                foreach ($deltas as $delta) {
                    yield $delta;
                }
            };

            yield new Result(new StreamResult($gen()));
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
