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
interface ArchetypeSelectorInterface
{
    /**
     * Analyzes a job description and selects the most suitable archetype.
     *
     * @param string $jobDescription The job description text
     */
    public function select(string $jobDescription): ArchetypeSelection;
}
