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

namespace App\Tests\AI\Agent;

use App\AI\Agent\ResumeDrafter;
use App\AI\Agent\ResumeDraftRequest;
use PHPUnit\Framework\Attributes\CoversClass;
use PHPUnit\Framework\Attributes\DataProvider;
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
#[CoversClass(ResumeDrafter::class)]
final class ResumeDrafterTest extends TestCase
{
    #[Test]
    public function itDraftsResumeWithBothJobDescriptionAndCandidateArchetype(): void
    {
        $jobDescription = "Role: Staff Backend Engineer\nTech: PHP, Symfony";
        $candidateArchetype = 'Staff Engineer / Technical Leader';
        $expectedOutput = "# Jane Doe\n\nStaff Engineer";

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $expectedOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($expectedOutput);
            });

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draft($jobDescription, $candidateArchetype);

        $this->assertSame($expectedOutput, $result);
        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString('Synthesize a complete, high-impact resume in Markdown format based on the candidate profile.', $capturedPrompt);
        $this->assertStringContainsString("<job_description>\n{$jobDescription}\n</job_description>", $capturedPrompt);
        $this->assertStringContainsString("<candidate_archetype>\n{$candidateArchetype}\n</candidate_archetype>", $capturedPrompt);
        $this->assertStringContainsString('Output only the final Markdown resume. Do not include introductory text, explanations, or enclosing code block fences.', $capturedPrompt);
    }

    #[Test]
    public function itDraftsResumeWithJobDescriptionOnly(): void
    {
        $jobDescription = "Role: Backend Engineer\nTech: Symfony";
        $expectedOutput = "# Jane Doe\n\nBackend Engineer";

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $expectedOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($expectedOutput);
            });

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draft($jobDescription);

        $this->assertSame($expectedOutput, $result);
        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString("<job_description>\n{$jobDescription}\n</job_description>", $capturedPrompt);
        $this->assertStringNotContainsString('<candidate_archetype>', $capturedPrompt);
    }

    #[Test]
    public function itDraftsResumeWithCandidateArchetypeOnly(): void
    {
        $candidateArchetype = 'Engineering Manager';
        $expectedOutput = "# Jane Doe\n\nEngineering Manager";

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $expectedOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($expectedOutput);
            });

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draft(null, $candidateArchetype);

        $this->assertSame($expectedOutput, $result);
        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString("<candidate_archetype>\n{$candidateArchetype}\n</candidate_archetype>", $capturedPrompt);
        $this->assertStringNotContainsString('<job_description>', $capturedPrompt);
    }

    #[Test]
    public function itDraftsResumeWithNoContext(): void
    {
        $expectedOutput = "# Jane Doe\n\nGeneral Resume";

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $expectedOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($expectedOutput);
            });

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draft();

        $this->assertSame($expectedOutput, $result);
        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString('Synthesize a complete, high-impact resume in Markdown format based on the candidate profile.', $capturedPrompt);
        $this->assertStringNotContainsString('<job_description>', $capturedPrompt);
        $this->assertStringNotContainsString('<candidate_archetype>', $capturedPrompt);
    }

    #[Test]
    public function itDraftsFromRequestObject(): void
    {
        $request = new ResumeDraftRequest(
            jobDescription: 'Software Architect',
            candidateArchetype: 'Principal Architect',
        );
        $expectedOutput = "# Jane Doe\n\nPrincipal Architect";

        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt, $expectedOutput): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution($expectedOutput);
            });

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draftFromRequest($request);

        $this->assertSame($expectedOutput, $result);
        $this->assertNotNull($capturedPrompt);
        $this->assertStringContainsString('<job_description>', $capturedPrompt);
        $this->assertStringContainsString('<candidate_archetype>', $capturedPrompt);
    }

    #[Test]
    public function itTreatsEmptyAndWhitespaceOnlyStringsAsNull(): void
    {
        $capturedPrompt = null;
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturnCallback(function (string $prompt) use (&$capturedPrompt): Execution {
                $capturedPrompt = $prompt;

                return $this->createExecution('# Resume');
            });

        $drafter = new ResumeDrafter($agent);
        $drafter->draft('   ', '');

        $this->assertNotNull($capturedPrompt);
        $this->assertStringNotContainsString('<job_description>', $capturedPrompt);
        $this->assertStringNotContainsString('<candidate_archetype>', $capturedPrompt);
    }

    #[Test]
    #[DataProvider('provideSanitizationCases')]
    public function itSanitizesAgentOutput(string $rawOutput, string $expectedCleanOutput): void
    {
        $agent = $this->createMock(AgentInterface::class);
        $agent->expects($this->once())
            ->method('call')
            ->willReturn($this->createExecution($rawOutput));

        $drafter = new ResumeDrafter($agent);
        $result = $drafter->draft();

        $this->assertSame($expectedCleanOutput, $result);
    }

    /**
     * @return iterable<string, array{string, string}>
     */
    public static function provideSanitizationCases(): iterable
    {
        yield 'plain markdown' => [
            "# John Doe\n\nSoftware Engineer",
            "# John Doe\n\nSoftware Engineer",
        ];

        yield 'wrapped in markdown code fence' => [
            "```markdown\n# John Doe\n\nSoftware Engineer\n```",
            "# John Doe\n\nSoftware Engineer",
        ];

        yield 'wrapped in generic code fence' => [
            "```\n# John Doe\n\nSoftware Engineer\n```",
            "# John Doe\n\nSoftware Engineer",
        ];

        yield 'surrounding whitespace and newlines' => [
            "  \n\n```markdown\n# John Doe\n```\n\n  ",
            '# John Doe',
        ];

        yield 'plain with leading and trailing whitespace' => [
            "  \n# John Doe\n\n  ",
            '# John Doe',
        ];
    }

    private function createExecution(string $output): Execution
    {
        return new Execution(static function () use ($output): \Generator {
            yield new Result(new TextResult($output));
        });
    }
}
