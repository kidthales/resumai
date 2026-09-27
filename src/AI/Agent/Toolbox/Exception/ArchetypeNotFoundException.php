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

namespace App\AI\Agent\Toolbox\Exception;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final class ArchetypeNotFoundException extends \InvalidArgumentException
{
    public static function forFilename(string $filename): self
    {
        return new self(\sprintf('Archetype with filename "%s" was not found.', $filename));
    }
}
