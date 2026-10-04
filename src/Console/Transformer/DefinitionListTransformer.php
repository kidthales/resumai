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

namespace App\Console\Transformer;

use Symfony\Component\Serializer\Normalizer\NormalizerInterface;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
final readonly class DefinitionListTransformer
{
    public function __construct(private NormalizerInterface $normalizer)
    {
    }

    /**
     * @throws \Symfony\Component\Serializer\Exception\ExceptionInterface
     */
    public function transform(mixed $data, array $context = []): array
    {
        $normalized = $this->normalizer->normalize($data, null, $context);

        if (null === $normalized || is_scalar($normalized)) {
            return [$normalized];
        }

        $flattened = $this->flatten($normalized);

        $definitionList = [];

        foreach ($flattened as $key => $value) {
            $definitionList[] = [$key => $value];
        }

        return $definitionList;
    }

    private function flatten(array|\ArrayObject $data, string $keyPrefix = ''): array
    {
        $flattened = [];

        foreach ($data as $key => $value) {
            $flattenedKey = is_int($key)
                ? $keyPrefix.'['.$key.']'
                : ((empty($keyPrefix) ? '' : ($keyPrefix.'.')).$key);

            if (is_array($value) || $value instanceof \ArrayObject) {
                $flattened = [...$flattened, ...$this->flatten($value, $flattenedKey)];
                continue;
            }

            $flattened[$flattenedKey] = $value;
        }

        return $flattened;
    }
}
