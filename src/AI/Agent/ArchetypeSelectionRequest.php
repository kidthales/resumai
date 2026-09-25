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
final readonly class ArchetypeSelectionRequest
{
    /**
     * @param (callable(ThinkingDelta):void)|null $thinkingDeltaProcessor
     * @param (callable(TextDelta):void)|null     $textDeltaProcessor
     */
    public function __construct(
        public string $jobDescription,
        public mixed $thinkingDeltaProcessor = null,
        public mixed $textDeltaProcessor = null,
    ) {
    }
}
