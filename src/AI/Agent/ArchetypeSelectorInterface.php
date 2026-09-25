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

namespace App\AI\Agent;

use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
interface ArchetypeSelectorInterface
{
    /**
     * Analyzes a job description and selects the most suitable archetype.
     *
     * @param string                              $jobDescription         The job description text
     * @param (callable(ThinkingDelta):void)|null $thinkingDeltaProcessor Optional callback for thinking deltas
     * @param (callable(TextDelta):void)|null     $textDeltaProcessor     Optional callback for text deltas
     */
    public function select(
        string $jobDescription,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $textDeltaProcessor = null,
    ): ArchetypeSelection;

    /**
     * Analyzes a job description using a request object.
     */
    public function selectFromRequest(ArchetypeSelectionRequest $request): ArchetypeSelection;
}
