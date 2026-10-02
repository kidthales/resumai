<?php

declare(strict_types=1);

namespace App\Console\Style;

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
