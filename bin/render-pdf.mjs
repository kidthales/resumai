#!/usr/bin/env node

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

import {resolve} from 'node:path';
import puppeteer from 'puppeteer';

if (4 !== process.argv.length) {
    console.error('Usage: node to-pdf.mjs <input-path> <output-path>');
    process.exit(1);
}

const inputPath = process.argv[2];
const outputPath = process.argv[3];

async function renderPdf() {
    const browser = await puppeteer.launch({
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu']
    });

    try {
        const page = await browser.newPage();
        await page.goto(`file://${resolve(inputPath)}`, {waitUntil: 'networkidle0'});

        await page.pdf({
            path: resolve(outputPath),
            format: 'Letter',
            printBackground: true,
            preferCSSPageSize: true,
            margin: {
                top: '0.4in',
                right: '0.5in',
                bottom: '0.4in',
                left: '0.5in'
            }
        });
    } finally {
        await browser.close();
    }
}

renderPdf().catch((err) => {
    console.error('PDF rendering failed:', err);
    process.exit(1);
});
