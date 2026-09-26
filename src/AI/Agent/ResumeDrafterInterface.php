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

namespace App\AI\Agent;

use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
interface ResumeDrafterInterface
{
    /**
     * Synthesizes an ATS-optimized resume in Markdown format.
     *
     * @param string|null                         $jobDescription         Optional target job description
     * @param string|null                         $candidateArchetype     Optional candidate positioning archetype
     * @param (callable(ThinkingDelta):void)|null $thinkingDeltaProcessor Optional callback for thinking deltas
     * @param (callable(TextDelta):void)|null     $textDeltaProcessor     Optional callback for text deltas
     *
     * @return string Raw Markdown resume content
     */
    public function draft(
        ?string $jobDescription = null,
        ?string $candidateArchetype = null,
        ?callable $thinkingDeltaProcessor = null,
        ?callable $textDeltaProcessor = null,
    ): string;
}
