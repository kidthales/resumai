<?php

declare(strict_types=1);

namespace App\Command;

use Symfony\AI\Agent\Execution\Execution;
use Symfony\AI\Agent\Execution\Update\Progress;
use Symfony\AI\Platform\Result\Stream\Delta\TextDelta;
use Symfony\AI\Platform\Result\Stream\Delta\ThinkingDelta;
use Symfony\Component\Console\Helper\ProgressIndicator;

/**
 * @author Tristan Bonsor <kidthales@agogpixel.com>
 */
trait StreamExecutionProgressTrait
{
    protected function streamExecutionProgress(
        Execution $execution,
        ProgressIndicator $indicator,
        string &$resultText,
        string &$thinkingText,
    ): void {
        $execution->onProgress(function (Progress $progress) use ($indicator) {
            $indicator->advance();
            $indicator->setMessage($progress->getMessage());
        });

        $resultText = '';
        $thinkingText = '';
        foreach ($execution->asStream() as $delta) {
            $indicator->advance();

            if ($delta instanceof ThinkingDelta) {
                $thinkingText .= $delta->getThinking();

                $preview = str_replace("\n", ' ', mb_substr($thinkingText, -40));
                $indicator->setMessage(sprintf('Thinking: "...%s"', $preview));
            } elseif ($delta instanceof TextDelta) {
                $resultText .= $delta->getText();

                $preview = str_replace("\n", ' ', mb_substr($resultText, -40));
                $indicator->setMessage(sprintf('Generating: "...%s"', $preview));
            }
        }

        $indicator->finish('<info>Execution completed.</info>');
    }
}
