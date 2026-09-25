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

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class ArchetypeSelection
{
    public function __construct(
        public string $archetypeId,
        public string $archetypeName,
        public string $content,
        public ?string $rationale = null,
    ) {
    }
}
